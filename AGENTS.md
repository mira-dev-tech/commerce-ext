# AGENTS.md — commerce-ext

Este repositório é o contrato público que um plugin de parceiro compila.
A fonte do protocolo está no monorepo interno; aqui chega por sincronização.
Quem escreve um plugin importa este módulo e mais nada da plataforma.

Leia isto antes de editar um plugin ou de «ajudar a publicar».

## O que este módulo é

`github.com/mira-dev-tech/commerce-ext` é a única dependência de plataforma
permitida. O `main` do plugin é `commerceext.Serve`. O processo escuta
`--commerce-ext-rpc` e o host da Mirá disca. O handshake
`--commerce-ext-handshake` imprime `commerce-ext-ok` e termina: é a prova de
que o binário fala o protocolo, não é o deploy.

## Regras

1. Não clone o monorepo (`mira-arandu-core`, `mira-commerce-core`) e não
   importe `internal/`. A razão: o parceiro não pode herdar credencial, mapa
   de tenants nem um caminho que a revisão do core não viu.

2. Não peça nem grave `DATABASE_URL`, `TENANT_DATABASES`, `TENANT_MEMBERS`
   nem uma DSN. A razão: o processo do plugin não liga ao Postgres. Quem
   migra o schema `ext_<id>` e abre a transação é o host, com um role sem
   superuser e sem `BYPASSRLS`. O nome do schema deriva do id
   (`alfa-fidelidade` → `ext_alfa_fidelidade`). O plugin não escolhe o schema.

3. Não use `MIRA_MANIFEST_PATH` nem descreva o plugin como filho da API.
   A razão: esse caminho arranca o binário dentro do processo que serve os
   outros tenants e dá-lhe os privilégios do core. O plugin de parceiro é
   outro pod. A única env que ele recebe no cluster é `COMMERCE_EXT_HOST`,
   o endereço do host. Desligar a release não reinicia a API.

4. Não mande preço nem chave de idempotência no hook. `Usage.Add` leva o
   medidor, quantas vezes e a chave de negócio. A razão: o preço é o da
   release que a empresa aceitou, e a chave de idempotência é do host
   (empresa, plugin, versão, medidor, chave de negócio). Um valor vindo do
   processo não entra no débito. Sem saldo o hook não corre. Timeout ou
   panic depois do débito estorna. Um «não» de negócio não estorna.

5. Não grave a release e não marque o pin como `active`.
   `POST /platform/plugin-releases` recusa a sessão de parceiro (401).
   A razão: o digest é o da imagem que a pipeline da Mirá construiu a partir
   da tag, não um digest declarado no repo. O install grava `installing`.
   Só passa a `active` quando o host diz que o role respeita a RLS.

6. Não instale numa empresa que o registo do parceiro não liste. A razão:
   o `Registry` é a única resposta a «esta empresa é deste parceiro». Noutra
   empresa o host nem abre o RPC.

7. Não publique neste espelho segredo, host de cluster, nome de cliente nem
   UUID real. A razão: o repositório é público. Issues sim, push de protocolo
   só pela sincronização da Mirá.

8. Hook síncrono tem 300ms. Sem I/O bloqueante. Código de bloqueio, uma vez
   publicado, não se renomeia: front e suporte dependem dele. Evento é
   at-least-once: a idempotência é a chave de negócio, não o id do evento.

## Publicar, só pelo GitHub

1. PR no repo do plugin. O check é `go test ./...` mais o handshake. O
   `plugintest` é o host mínimo: cadeia, timeout, wallet falsa, base local.
   Se passa aqui e falha no host real, o desvio é da Mirá.

2. Tag `vX.Y.Z` no commit verde. Não se envia binário.

3. O workflow da plataforma clona esse SHA, constrói a imagem, calcula o digest e
   grava a release (id, versão, `partner_id`, digest, `compatibleCore`, preço
   dos medidores). O `partner_id` tem de estar ativo no registo dinâmico do
   Console. Um `plugin_id` que já pertence a outro parceiro é recusado na
   gravação. Regravar a mesma versão é recusado.

4. Ligar a uma empresa é outro disparo, com o token de parceiro e o slug.
   O pin fica `installing` até o role `ext_<id>` existir e a Mirá activar.

5. Suspender é um disparo da Mirá. Tira a release de todas as empresas dele
   e não reinicia a API.

O `partner_id` é o do registo dinâmico do Console, não um ficheiro. Sem ele
ativo a tag não grava.
