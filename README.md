# commerce-ext — SDK público de plugins do Mira Commerce

Módulo Go que define o **Extension Protocol** do mira-commerce-core: a interface
`Plugin`, o catálogo de hooks e eventos, os tipos de input/output, o formato do
`manifest.yaml` e o runtime `Serve` para plugins externos (go-plugin/RPC).

**É a única dependência que um plugin pode ter do core.** Plugins nunca
importam `mira-commerce-core/internal/...`.

```go
import commerceext "github.com/mira-dev-tech/commerce-ext"
```

## Fonte de verdade e versionamento

- **SSOT**: diretório [`commerce-ext/`](https://github.com/mira-dev-tech/mira-commerce-core/tree/main/commerce-ext)
  do `mira-commerce-core`. Este repositório é um **espelho publicado** para que
  plugins fora do monorepo consigam resolver o módulo — mudanças entram
  primeiro no core e são sincronizadas para cá manualmente (por ora).
- Tags seguem a linha do protocolo (`CoreLine` em `version.go`): `v0.2.0` ↔
  core API v0.2. O `compatibleCore` do manifest do plugin é validado contra
  essa linha (semver).

## Como começar um plugin

Use o template [`mira-commerce-plugin-example`](https://github.com/mira-dev-tech/mira-commerce-plugin-example)
e siga o guia [`docs/plugin-example.md`](https://github.com/mira-dev-tech/mira-commerce-core/blob/main/docs/plugin-example.md)
do core.

## Acesso (módulo privado)

```bash
export GOPRIVATE=github.com/mira-dev-tech
# com SSH configurado para o GitHub:
git config --global url."git@github.com:".insteadOf "https://github.com/"
```
