package plugintest

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	commerceext "github.com/mira-dev-tech/commerce-ext"
)

// ErrTenantMissing recusa a chamada sem tenant. O mesmo critério do host: sem
// tenant não há linha, e não se inventa uma.
var ErrTenantMissing = errors.New("plugintest: tenant ausente")

// ErrUnsupportedSQL é SQL que a guarda aceita e este motor local não corre.
// Não é ErrForbiddenStatement: a instrução é DML legítimo, só não está no
// subconjunto (INSERT/SELECT/UPDATE/DELETE simples, com $n, now() e
// ON CONFLICT DO UPDATE).
var ErrUnsupportedSQL = errors.New("plugintest: SQL fora do subconjunto local")

type colDef struct {
	name    string
	kind    string // "", "tenant", "now", "uuid"
	literal string
}

type memTable struct {
	cols []colDef
	rows []map[string]any
}

// DB é o PluginDB em memória. A guarda é a do host. O tenant da construção
// entra em toda a linha e filtra toda a leitura, mesmo quando o SQL não o
// menciona — é o lugar da RLS neste teste.
type DB struct {
	mu     *sync.Mutex
	schema string
	tenant string
	tables map[string]*memTable
}

// NewDB abre o schema do plugin para um tenant. O schema tem de ser ext_*.
func NewDB(schema, tenant string) (*DB, error) {
	schema = strings.TrimSpace(schema)
	tenant = strings.TrimSpace(tenant)
	if !strings.HasPrefix(schema, commerceext.SchemaPrefix) || strings.ContainsAny(schema, ` ."'\`) {
		return nil, fmt.Errorf("plugintest: schema %q inválido", schema)
	}
	if tenant == "" {
		return nil, fmt.Errorf("plugintest: tenant vazio")
	}
	return &DB{
		mu: &sync.Mutex{}, schema: schema, tenant: tenant,
		tables: map[string]*memTable{},
	}, nil
}

// WithTenant devolve o mesmo schema visto por outro tenant. Serve para provar
// que uma linha de A não aparece em B.
func (d *DB) WithTenant(tenant string) *DB {
	if d == nil {
		return nil
	}
	return &DB{mu: d.mu, schema: d.schema, tenant: strings.TrimSpace(tenant), tables: d.tables}
}

// Schema implementa commerceext.PluginDB.
func (d *DB) Schema() string {
	if d == nil {
		return ""
	}
	return d.schema
}

// Exec implementa commerceext.PluginDB.
func (d *DB) Exec(_ context.Context, sql string, args ...any) (int64, error) {
	if d == nil {
		return 0, commerceext.ErrNotProvisioned
	}
	if err := commerceext.GuardStatement(d.schema, sql); err != nil {
		return 0, err
	}
	if strings.TrimSpace(d.tenant) == "" {
		return 0, ErrTenantMissing
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.execLocked(sql, args)
}

// Query implementa commerceext.PluginDB.
func (d *DB) Query(_ context.Context, sql string, args ...any) (commerceext.Rows, error) {
	if d == nil {
		return nil, commerceext.ErrNotProvisioned
	}
	if err := commerceext.GuardStatement(d.schema, sql); err != nil {
		return nil, err
	}
	if strings.TrimSpace(d.tenant) == "" {
		return nil, ErrTenantMissing
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.queryLocked(sql, args)
}

// QueryRow implementa commerceext.PluginDB.
func (d *DB) QueryRow(ctx context.Context, sql string, args ...any) commerceext.Row {
	rows, err := d.Query(ctx, sql, args...)
	if err != nil {
		return errRow{err}
	}
	defer rows.Close()
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return errRow{err}
		}
		return errRow{commerceext.ErrNoRows}
	}
	b, ok := rows.(*memRows)
	if !ok || b.idx < 0 || b.idx >= len(b.rows) {
		return errRow{commerceext.ErrNoRows}
	}
	return &valueRow{values: b.rows[b.idx]}
}

// Migrate aplica os `-- +goose Up` de dir, por ordem de nome. CREATE TABLE
// regista colunas. Índice, RLS, policy e GRANT são saltados: aqui quem recorta
// o tenant é o acessor, não uma policy. Outro DDL devolve erro com a frase.
func (d *DB) Migrate(dir string) error {
	if d == nil {
		return commerceext.ErrNotProvisioned
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".sql") {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)
	d.mu.Lock()
	defer d.mu.Unlock()
	for _, name := range names {
		raw, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			return err
		}
		up := gooseUp(string(raw))
		for _, stmt := range splitSQL(up) {
			if err := d.applyDDL(stmt); err != nil {
				return fmt.Errorf("plugintest: %s: %w", name, err)
			}
		}
	}
	return nil
}

func (d *DB) applyDDL(stmt string) error {
	s := strings.TrimSpace(stmt)
	low := strings.ToLower(s)
	switch {
	case s == "":
		return nil
	case strings.HasPrefix(low, "create table"):
		return d.createTable(s)
	case strings.HasPrefix(low, "create index"), strings.HasPrefix(low, "create unique index"),
		strings.HasPrefix(low, "comment "), strings.HasPrefix(low, "grant "),
		strings.HasPrefix(low, "revoke "), strings.HasPrefix(low, "create policy"),
		strings.HasPrefix(low, "drop policy"):
		return nil
	case strings.HasPrefix(low, "alter table") && strings.Contains(low, "row level security"):
		return nil
	case strings.HasPrefix(low, "alter table") && strings.Contains(low, "add column"):
		return d.addColumn(s)
	default:
		return fmt.Errorf("migração não suportada no harness local: %.80s", oneLine(s))
	}
}

func (d *DB) createTable(stmt string) error {
	name, body, err := tableBody(stmt)
	if err != nil {
		return err
	}
	if _, ok := d.tables[name]; ok {
		return nil
	}
	cols, err := parseColumns(body)
	if err != nil {
		return err
	}
	d.tables[name] = &memTable{cols: cols}
	return nil
}

func (d *DB) addColumn(stmt string) error {
	low := strings.ToLower(stmt)
	i := strings.Index(low, "add column")
	if i < 0 {
		return fmt.Errorf("alter table ilegível")
	}
	head := strings.TrimSpace(low[:i])
	fields := strings.Fields(head)
	if len(fields) < 3 {
		return fmt.Errorf("alter table sem nome")
	}
	table := fields[2]
	t, ok := d.tables[table]
	if !ok {
		return fmt.Errorf("tabela %q não existe", table)
	}
	rest := strings.TrimSpace(low[i+len("add column"):])
	rest = strings.TrimSpace(strings.TrimPrefix(rest, "if not exists"))
	cols, err := parseColumns(rest)
	if err != nil || len(cols) != 1 {
		return fmt.Errorf("add column ilegível")
	}
	for _, c := range t.cols {
		if c.name == cols[0].name {
			return nil
		}
	}
	t.cols = append(t.cols, cols[0])
	return nil
}

func tableBody(stmt string) (string, string, error) {
	low := strings.ToLower(stmt)
	rest := strings.TrimSpace(stmt[len("create table"):])
	lowRest := strings.ToLower(rest)
	if strings.HasPrefix(lowRest, "if not exists") {
		rest = strings.TrimSpace(rest[len("if not exists"):])
	}
	fields := strings.Fields(rest)
	if len(fields) == 0 {
		return "", "", fmt.Errorf("create table sem nome")
	}
	name := strings.ToLower(strings.Trim(fields[0], `"`))
	open := strings.IndexByte(stmt, '(')
	if open < 0 || !strings.Contains(low, "create table") {
		return "", "", fmt.Errorf("create table sem colunas")
	}
	body, err := matchingParen(stmt, open)
	if err != nil {
		return "", "", err
	}
	return name, body, nil
}

func matchingParen(s string, open int) (string, error) {
	depth := 0
	for i := open; i < len(s); i++ {
		switch s[i] {
		case '(':
			depth++
		case ')':
			depth--
			if depth == 0 {
				return s[open+1 : i], nil
			}
		}
	}
	return "", fmt.Errorf("parêntese sem fecho")
}

func parseColumns(body string) ([]colDef, error) {
	parts := splitDepth(body, ',')
	var cols []colDef
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		word := strings.ToLower(strings.Fields(p)[0])
		switch word {
		case "primary", "unique", "constraint", "check", "foreign", "exclude":
			continue
		}
		name := strings.ToLower(strings.Trim(word, `"`))
		low := strings.ToLower(p)
		c := colDef{name: name}
		switch {
		case strings.Contains(low, "current_setting"):
			c.kind = "tenant"
		case strings.Contains(low, "gen_random_uuid"):
			c.kind = "uuid"
		case strings.Contains(low, "now()"):
			c.kind = "now"
		default:
			if i := strings.Index(low, "default"); i >= 0 {
				c.kind = "literal"
				c.literal = strings.Trim(strings.TrimSpace(p[i+len("default"):]), "'")
				if sp := strings.IndexAny(c.literal, " \t"); sp >= 0 {
					c.literal = c.literal[:sp]
				}
			}
		}
		cols = append(cols, c)
	}
	if len(cols) == 0 {
		return nil, fmt.Errorf("tabela sem colunas")
	}
	return cols, nil
}

func (d *DB) execLocked(sql string, args []any) (int64, error) {
	norm := normalizeSQL(sql)
	low := strings.ToLower(strings.TrimSpace(norm))
	switch {
	case strings.HasPrefix(low, "insert"):
		return d.insert(norm, args)
	case strings.HasPrefix(low, "update"):
		return d.update(norm, args)
	case strings.HasPrefix(low, "delete"):
		return d.delete(norm, args)
	default:
		return 0, fmt.Errorf("%w: %.80s", ErrUnsupportedSQL, oneLine(norm))
	}
}

func (d *DB) queryLocked(sql string, args []any) (commerceext.Rows, error) {
	norm := normalizeSQL(sql)
	low := strings.ToLower(strings.TrimSpace(norm))
	if !strings.HasPrefix(low, "select") {
		return nil, fmt.Errorf("%w: %.80s", ErrUnsupportedSQL, oneLine(norm))
	}
	return d.querySelect(norm, args)
}

func normalizeSQL(sql string) string {
	s := strings.TrimSpace(sql)
	s = strings.TrimSuffix(s, ";")
	for {
		i := strings.Index(s, "::")
		if i < 0 {
			break
		}
		j := i + 2
		for j < len(s) && isIdent(s[j]) {
			j++
		}
		s = s[:i] + s[j:]
	}
	return strings.TrimSpace(s)
}

func isIdent(c byte) bool {
	return c == '_' || (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9')
}

func (d *DB) insert(sql string, args []any) (int64, error) {
	low := strings.ToLower(sql)
	on := strings.Index(low, "on conflict")
	base := sql
	var conflict []string
	var sets string
	if on >= 0 {
		base = sql[:on]
		rest := sql[on:]
		open := strings.IndexByte(rest, '(')
		if open < 0 {
			return 0, fmt.Errorf("%w: on conflict sem colunas", ErrUnsupportedSQL)
		}
		body, err := matchingParen(rest, open)
		if err != nil {
			return 0, err
		}
		for _, c := range splitDepth(body, ',') {
			conflict = append(conflict, strings.ToLower(strings.TrimSpace(c)))
		}
		up := strings.Index(strings.ToLower(rest), "do update set")
		if up < 0 {
			return 0, fmt.Errorf("%w: on conflict sem do update", ErrUnsupportedSQL)
		}
		sets = strings.TrimSpace(rest[up+len("do update set"):])
	}
	into := strings.Index(strings.ToLower(base), "into")
	if into < 0 {
		return 0, fmt.Errorf("%w: insert sem into", ErrUnsupportedSQL)
	}
	open := strings.IndexByte(base[into:], '(')
	if open < 0 {
		return 0, fmt.Errorf("%w: insert sem colunas", ErrUnsupportedSQL)
	}
	open += into
	colsBody, err := matchingParen(base, open)
	if err != nil {
		return 0, err
	}
	valAt := strings.Index(strings.ToLower(base), "values")
	if valAt < 0 {
		return 0, fmt.Errorf("%w: insert sem values", ErrUnsupportedSQL)
	}
	vopen := strings.IndexByte(base[valAt:], '(')
	if vopen < 0 {
		return 0, fmt.Errorf("%w: insert sem values", ErrUnsupportedSQL)
	}
	valsBody, err := matchingParen(base, valAt+vopen)
	if err != nil {
		return 0, err
	}
	name := strings.ToLower(strings.Fields(base[into+len("into"):])[0])
	t, ok := d.tables[name]
	if !ok {
		return 0, fmt.Errorf("plugintest: tabela %q não existe — corre Migrate", name)
	}
	colNames := splitIdents(colsBody)
	vals := splitDepth(valsBody, ',')
	if len(colNames) != len(vals) {
		return 0, fmt.Errorf("plugintest: insert com %d colunas e %d valores", len(colNames), len(vals))
	}
	row := map[string]any{}
	for i, c := range colNames {
		if c == "tenant_id" {
			continue
		}
		v, err := evalExpr(strings.TrimSpace(vals[i]), args, nil)
		if err != nil {
			return 0, err
		}
		row[c] = v
	}
	d.fillDefaults(t, row)
	if len(conflict) > 0 {
		if idx := findConflict(t, conflict, row); idx >= 0 {
			if err := applyUpdate(t.rows[idx], sets, args, row); err != nil {
				return 0, err
			}
			return 1, nil
		}
	}
	t.rows = append(t.rows, row)
	return 1, nil
}

func (d *DB) fillDefaults(t *memTable, row map[string]any) {
	for _, c := range t.cols {
		if _, ok := row[c.name]; ok {
			continue
		}
		switch c.kind {
		case "tenant":
			row[c.name] = d.tenant
		case "now":
			row[c.name] = time.Now().UTC()
		case "uuid":
			row[c.name] = newUUID()
		case "literal":
			row[c.name] = c.literal
		}
	}
	if _, ok := row["tenant_id"]; !ok {
		for _, c := range t.cols {
			if c.name == "tenant_id" {
				row["tenant_id"] = d.tenant
			}
		}
	} else {
		row["tenant_id"] = d.tenant
	}
}

func findConflict(t *memTable, cols []string, row map[string]any) int {
	for i, existing := range t.rows {
		match := true
		for _, c := range cols {
			if fmt.Sprint(existing[c]) != fmt.Sprint(row[c]) {
				match = false
				break
			}
		}
		if match {
			return i
		}
	}
	return -1
}

func applyUpdate(row map[string]any, sets string, args []any, excluded map[string]any) error {
	for _, asg := range splitDepth(sets, ',') {
		asg = strings.TrimSpace(asg)
		eq := strings.IndexByte(asg, '=')
		if eq < 0 {
			return fmt.Errorf("%w: set sem igual", ErrUnsupportedSQL)
		}
		col := strings.ToLower(strings.TrimSpace(asg[:eq]))
		if col == "tenant_id" {
			row["tenant_id"] = row["tenant_id"]
			continue
		}
		v, err := evalExpr(strings.TrimSpace(asg[eq+1:]), args, excluded)
		if err != nil {
			return err
		}
		row[col] = v
	}
	return nil
}

func (d *DB) update(sql string, args []any) (int64, error) {
	low := strings.ToLower(sql)
	setAt := strings.Index(low, " set ")
	whereAt := strings.Index(low, " where ")
	if setAt < 0 || whereAt < setAt {
		return 0, fmt.Errorf("%w: update sem where", ErrUnsupportedSQL)
	}
	name := strings.ToLower(strings.Fields(sql)[1])
	t, ok := d.tables[name]
	if !ok {
		return 0, fmt.Errorf("plugintest: tabela %q não existe", name)
	}
	sets := sql[setAt+len(" set ") : whereAt]
	where := sql[whereAt+len(" where "):]
	var n int64
	for _, row := range t.rows {
		ok, err := d.match(row, where, args)
		if err != nil {
			return 0, err
		}
		if !ok {
			continue
		}
		if err := applyUpdate(row, sets, args, nil); err != nil {
			return 0, err
		}
		row["tenant_id"] = d.tenant
		n++
	}
	return n, nil
}

func (d *DB) delete(sql string, args []any) (int64, error) {
	low := strings.ToLower(sql)
	from := strings.Index(low, "from")
	whereAt := strings.Index(low, " where ")
	if from < 0 || whereAt < 0 {
		return 0, fmt.Errorf("%w: delete sem where", ErrUnsupportedSQL)
	}
	name := strings.ToLower(strings.Fields(sql[from+len("from"):])[0])
	t, ok := d.tables[name]
	if !ok {
		return 0, fmt.Errorf("plugintest: tabela %q não existe", name)
	}
	where := sql[whereAt+len(" where "):]
	var kept []map[string]any
	var n int64
	for _, row := range t.rows {
		ok, err := d.match(row, where, args)
		if err != nil {
			return 0, err
		}
		if ok {
			n++
			continue
		}
		kept = append(kept, row)
	}
	t.rows = kept
	return n, nil
}

func (d *DB) querySelect(sql string, args []any) (commerceext.Rows, error) {
	low := strings.ToLower(sql)
	fromAt := strings.Index(low, " from ")
	if fromAt < 0 {
		return nil, fmt.Errorf("%w: select sem from", ErrUnsupportedSQL)
	}
	list := strings.TrimSpace(sql[len("select"):fromAt])
	rest := strings.TrimSpace(sql[fromAt+len(" from "):])
	where := ""
	name := rest
	if w := strings.Index(strings.ToLower(rest), " where "); w >= 0 {
		name = strings.TrimSpace(rest[:w])
		where = strings.TrimSpace(rest[w+len(" where "):])
	}
	name = strings.ToLower(strings.Fields(name)[0])
	t, ok := d.tables[name]
	if !ok {
		return nil, fmt.Errorf("plugintest: tabela %q não existe", name)
	}
	cols := splitDepth(list, ',')
	out := &memRows{idx: -1}
	for _, row := range t.rows {
		if fmt.Sprint(row["tenant_id"]) != d.tenant && rowHasTenant(t) {
			continue
		}
		if where != "" {
			ok, err := d.match(row, where, args)
			if err != nil {
				return nil, err
			}
			if !ok {
				continue
			}
		}
		var line []any
		for _, c := range cols {
			v, err := evalExpr(strings.TrimSpace(c), args, nil)
			if err != nil {
				if col, ok := asIdent(c); ok {
					line = append(line, row[col])
					continue
				}
				return nil, err
			}
			if col, ok := asIdent(c); ok {
				line = append(line, row[col])
				continue
			}
			line = append(line, v)
		}
		out.rows = append(out.rows, line)
		if len(out.rows) > 10_000 {
			return nil, fmt.Errorf("plugintest: consulta acima de 10000 linhas")
		}
	}
	return out, nil
}

func rowHasTenant(t *memTable) bool {
	for _, c := range t.cols {
		if c.name == "tenant_id" {
			return true
		}
	}
	return false
}

func (d *DB) match(row map[string]any, where string, args []any) (bool, error) {
	if fmt.Sprint(row["tenant_id"]) != d.tenant && row["tenant_id"] != nil {
		return false, nil
	}
	for _, term := range splitAnd(where) {
		term = strings.TrimSpace(term)
		low := strings.ToLower(term)
		if strings.Contains(low, "current_setting") {
			continue
		}
		eq := strings.IndexByte(term, '=')
		if eq < 0 {
			return false, fmt.Errorf("%w: filtro %q", ErrUnsupportedSQL, oneLine(term))
		}
		col, ok := asIdent(term[:eq])
		if !ok {
			return false, fmt.Errorf("%w: filtro %q", ErrUnsupportedSQL, oneLine(term))
		}
		if col == "tenant_id" {
			continue
		}
		want, err := evalExpr(strings.TrimSpace(term[eq+1:]), args, nil)
		if err != nil {
			return false, err
		}
		if fmt.Sprint(row[col]) != fmt.Sprint(want) {
			return false, nil
		}
	}
	return true, nil
}

func evalExpr(expr string, args []any, excluded map[string]any) (any, error) {
	e := strings.TrimSpace(expr)
	low := strings.ToLower(e)
	switch {
	case low == "now()":
		return time.Now().UTC(), nil
	case strings.HasPrefix(low, "excluded."):
		if excluded == nil {
			return nil, fmt.Errorf("%w: excluded fora de on conflict", ErrUnsupportedSQL)
		}
		return excluded[strings.ToLower(strings.TrimSpace(e[len("excluded."):]))], nil
	case strings.HasPrefix(e, "$"):
		n := 0
		for _, c := range e[1:] {
			if c < '0' || c > '9' {
				return nil, fmt.Errorf("%w: placeholder %q", ErrUnsupportedSQL, e)
			}
			n = n*10 + int(c-'0')
		}
		if n < 1 || n > len(args) {
			return nil, fmt.Errorf("plugintest: $%d sem argumento", n)
		}
		return args[n-1], nil
	case len(e) >= 2 && e[0] == '\'' && e[len(e)-1] == '\'':
		return strings.ReplaceAll(e[1:len(e)-1], "''", "'"), nil
	default:
		if col, ok := asIdent(e); ok {
			return col, fmt.Errorf("%w: expressão %q", ErrUnsupportedSQL, e)
		}
		return nil, fmt.Errorf("%w: expressão %q", ErrUnsupportedSQL, oneLine(e))
	}
}

func asIdent(s string) (string, bool) {
	s = strings.ToLower(strings.TrimSpace(s))
	if s == "" {
		return "", false
	}
	for i := 0; i < len(s); i++ {
		if !isIdent(s[i]) {
			return "", false
		}
	}
	return s, true
}

func splitIdents(s string) []string {
	var out []string
	for _, p := range splitDepth(s, ',') {
		if id, ok := asIdent(p); ok {
			out = append(out, id)
		}
	}
	return out
}

func splitAnd(where string) []string {
	low := strings.ToLower(where)
	var parts []string
	depth := 0
	start := 0
	for i := 0; i < len(where); i++ {
		switch where[i] {
		case '(':
			depth++
		case ')':
			if depth > 0 {
				depth--
			}
		}
		if depth == 0 && i+5 <= len(low) && low[i:i+5] == " and " {
			parts = append(parts, where[start:i])
			start = i + 5
			i += 4
		}
	}
	parts = append(parts, where[start:])
	return parts
}

func splitDepth(s string, sep byte) []string {
	var parts []string
	depth := 0
	start := 0
	inStr := false
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '\'':
			if inStr && i+1 < len(s) && s[i+1] == '\'' {
				i++
				continue
			}
			inStr = !inStr
		case '(':
			if !inStr {
				depth++
			}
		case ')':
			if !inStr && depth > 0 {
				depth--
			}
		}
		if !inStr && depth == 0 && s[i] == sep {
			parts = append(parts, s[start:i])
			start = i + 1
		}
	}
	parts = append(parts, s[start:])
	return parts
}

func gooseUp(sql string) string {
	const mark = "-- +goose Up"
	const down = "-- +goose Down"
	i := strings.Index(sql, mark)
	if i < 0 {
		return sql
	}
	sql = sql[i+len(mark):]
	if j := strings.Index(sql, down); j >= 0 {
		sql = sql[:j]
	}
	return sql
}

func splitSQL(sql string) []string {
	var parts []string
	depth := 0
	inStr := false
	start := 0
	for i := 0; i < len(sql); i++ {
		switch sql[i] {
		case '\'':
			if inStr && i+1 < len(sql) && sql[i+1] == '\'' {
				i++
				continue
			}
			inStr = !inStr
		case '(':
			if !inStr {
				depth++
			}
		case ')':
			if !inStr && depth > 0 {
				depth--
			}
		case ';':
			if !inStr && depth == 0 {
				parts = append(parts, sql[start:i])
				start = i + 1
			}
		}
	}
	if strings.TrimSpace(sql[start:]) != "" {
		parts = append(parts, sql[start:])
	}
	return parts
}

func oneLine(s string) string {
	return strings.Join(strings.Fields(s), " ")
}

func newUUID() string {
	var b [16]byte
	_, _ = rand.Read(b[:])
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

type memRows struct {
	rows [][]any
	idx  int
	err  error
}

func (m *memRows) Next() bool {
	if m.err != nil {
		return false
	}
	m.idx++
	return m.idx < len(m.rows)
}

func (m *memRows) Scan(dest ...any) error {
	if m.idx < 0 || m.idx >= len(m.rows) {
		return commerceext.ErrNoRows
	}
	return scanInto(m.rows[m.idx], dest)
}

func (m *memRows) Err() error { return m.err }
func (m *memRows) Close()     { m.idx = len(m.rows) }

type valueRow struct{ values []any }

func (r *valueRow) Scan(dest ...any) error { return scanInto(r.values, dest) }

type errRow struct{ err error }

func (r errRow) Scan(...any) error { return r.err }

func scanInto(vals []any, dest []any) error {
	if len(vals) != len(dest) {
		return fmt.Errorf("plugintest: scan de %d colunas para %d destinos", len(vals), len(dest))
	}
	for i, d := range dest {
		if err := assign(d, vals[i]); err != nil {
			return err
		}
	}
	return nil
}

func assign(dest, val any) error {
	switch p := dest.(type) {
	case *string:
		if p == nil {
			return fmt.Errorf("plugintest: destino nulo")
		}
		*p = fmt.Sprint(val)
		return nil
	case *int64:
		if p == nil {
			return fmt.Errorf("plugintest: destino nulo")
		}
		switch n := val.(type) {
		case int64:
			*p = n
		case int:
			*p = int64(n)
		default:
			return fmt.Errorf("plugintest: %T não é int64", val)
		}
		return nil
	case *[]byte:
		if p == nil {
			return fmt.Errorf("plugintest: destino nulo")
		}
		*p = []byte(fmt.Sprint(val))
		return nil
	default:
		return fmt.Errorf("plugintest: destino %T não suportado", dest)
	}
}
