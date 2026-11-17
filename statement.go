package commerceext

import (
	"fmt"
	"regexp"
	"strings"
)

// GuardStatement recusa a instrução que sai do schema do plugin (ADR 0227 D3).
//
// É a disciplina do contrato, partilhada pelo acessor do host
// (internal/ext/extdb) e pelo plugintest (ADR 0334 D2). Os dois chamam esta
// função para um SQL aceite aqui não ser recusado lá, e o contrário.
//
// O que ela é: allowlist do primeiro verbo. A primeira versão era blocklist de
// palavras e recusava `UPDATE t SET x = $1`, porque `SET` aparecia lá dentro.
//
// O que ela não é: a fronteira de segurança. Enquanto o plugin corre no
// processo da API, pode ignorar o acessor. O role do Postgres é que prende
// (0227 D5, 0333 D4).

// allowedVerbs é o que entra pelo acessor: DML e nada mais. DDL fica de fora
// porque schema é declaração versionada e não efeito de uma chamada; SET e
// RESET desfariam o search_path e o tenant; controlo de transacção é do
// acessor.
var allowedVerbs = map[string]bool{
	"select": true,
	"insert": true,
	"update": true,
	"delete": true,
	"with":   true,
	"values": true,
	"table":  true,
}

// forbiddenFunctions fazem, pelo nome, o que a allowlist impediria pela forma:
// mudar sessão, ler ficheiros, sair para outro banco, prender a conexão.
// set_config é o SET em roupa de SELECT.
var forbiddenFunctions = []string{
	"set_config",
	"pg_read_file", "pg_read_binary_file", "pg_ls_dir", "pg_stat_file",
	"lo_import", "lo_export",
	"dblink", "dblink_exec", "postgres_fdw_handler",
	"pg_sleep", "pg_sleep_for", "pg_sleep_until",
	"pg_terminate_backend", "pg_cancel_backend", "pg_reload_conf",
	"pg_logical_emit_message",
}

// reQualified apanha `algo.coisa`. Recusar toda a qualificação está errado:
// `FROM t r JOIN u a ON a.x = r.y` qualifica por alias. A regra é pelo nome do
// qualificador. Um alias chamado `core` é recusado, e isso é aceitável.
var reQualified = regexp.MustCompile(`(?i)\b([a-z_][a-z0-9_]*)\s*\.\s*[a-z_"*]`)

var blockedQualifiers = map[string]bool{
	"core":               true,
	"public":             true,
	"pg_catalog":         true,
	"pg_toast":           true,
	"pg_temp":            true,
	"information_schema": true,
}

var reFirstWord = regexp.MustCompile(`^[a-z]+`)

// GuardStatement recusa SQL que não seja DML no schema do plugin.
func GuardStatement(schema, sql string) error {
	stripped := stripLiteralsAndComments(sql)
	trimmed := strings.ToLower(strings.TrimSpace(stripped))
	trimmed = strings.TrimLeft(trimmed, "( \t\r\n")

	if trimmed == "" {
		return fmt.Errorf("%w: instrução vazia", ErrForbiddenStatement)
	}
	// Multi-statement é o caminho clássico para passar um verbo proibido atrás
	// de um SELECT inocente. O `;` final sozinho é tolerado.
	if strings.Contains(strings.TrimSuffix(strings.TrimSpace(stripped), ";"), ";") {
		return fmt.Errorf("%w: mais de uma instrução num só comando", ErrForbiddenStatement)
	}
	verb := reFirstWord.FindString(trimmed)
	if !allowedVerbs[verb] {
		return fmt.Errorf("%w: %q não entra pelo acessor do plugin — só SELECT/INSERT/UPDATE/DELETE/WITH (schema é migração, não chamada)",
			ErrForbiddenStatement, strings.ToUpper(verb))
	}
	for _, fn := range forbiddenFunctions {
		if containsWord(trimmed, fn) {
			return fmt.Errorf("%w: função %q não entra pelo acessor do plugin",
				ErrForbiddenStatement, fn)
		}
	}
	for _, m := range reQualified.FindAllStringSubmatch(stripped, -1) {
		qualifier := strings.ToLower(m[1])
		if strings.EqualFold(qualifier, schema) {
			continue
		}
		if blockedQualifiers[qualifier] || strings.HasPrefix(qualifier, SchemaPrefix) {
			return fmt.Errorf("%w: referência a %q — o acessor já fixa o search_path em %q, escreva a tabela sem qualificar",
				ErrForbiddenStatement, qualifier, schema)
		}
	}
	return nil
}

func containsWord(lowered, word string) bool {
	for i := 0; i+len(word) <= len(lowered); i++ {
		if lowered[i:i+len(word)] != word {
			continue
		}
		if i > 0 && isIdentChar(lowered[i-1]) {
			continue
		}
		if j := i + len(word); j < len(lowered) && isIdentChar(lowered[j]) {
			continue
		}
		return true
	}
	return false
}

func isIdentChar(c byte) bool {
	return c == '_' || c == '$' ||
		(c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9')
}

func stripLiteralsAndComments(sql string) string {
	var out strings.Builder
	out.Grow(len(sql))
	for i := 0; i < len(sql); {
		switch {
		case sql[i] == '\'':
			i = skipQuoted(sql, i, '\'', &out)
		case sql[i] == '"':
			i = skipQuoted(sql, i, '"', &out)
		case strings.HasPrefix(sql[i:], "--"):
			j := strings.IndexByte(sql[i:], '\n')
			if j < 0 {
				return out.String()
			}
			out.WriteByte('\n')
			i += j + 1
		case strings.HasPrefix(sql[i:], "/*"):
			j := strings.Index(sql[i+2:], "*/")
			if j < 0 {
				return out.String()
			}
			out.WriteByte(' ')
			i += 2 + j + 2
		default:
			out.WriteByte(sql[i])
			i++
		}
	}
	return out.String()
}

func skipQuoted(sql string, i int, quote byte, out *strings.Builder) int {
	out.WriteByte(quote)
	out.WriteByte(quote)
	i++
	for i < len(sql) {
		if sql[i] == quote {
			if i+1 < len(sql) && sql[i+1] == quote {
				i += 2
				continue
			}
			return i + 1
		}
		i++
	}
	return i
}
