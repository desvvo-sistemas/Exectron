# Contribuindo com o Exectron

Obrigado pelo interesse. Este documento resume o fluxo esperado — nada aqui é
burocrático, é só o combinado para as contribuições entrarem sem atrito.

## Antes de começar

Para mudanças grandes (novo recurso, refatoração ampla, mudança de formato dos
dados em disco), abra uma **issue** antes de escrever código. É mais barato
alinhar a ideia do que revisar um PR pronto que precisa mudar de rumo.

Correções de bug, ajustes de texto e melhorias pontuais podem ir direto em PR.

## Ambiente

| Ferramenta | Versão |
| --- | --- |
| [Go](https://go.dev/dl/) | 1.23 ou superior |
| [Node](https://nodejs.org) | 18 ou superior |
| [Wails CLI](https://wails.io/docs/gettingstarted/installation) | v2.12 |
| Docker | opcional — só para gerar o instalador Linux a partir do Windows |

```bash
go install github.com/wailsapp/wails/v2/cmd/wails@v2.12.0
wails doctor     # confere as dependências nativas da sua máquina
wails dev        # sobe o app com hot reload
```

No Linux, o build nativo precisa dos pacotes de desenvolvimento do GTK3, do
WebKit2GTK e do libayatana-appindicator:

```bash
sudo apt install build-essential pkg-config libgtk-3-dev libwebkit2gtk-4.1-dev libayatana-appindicator3-dev
```

## Antes de abrir o PR

```bash
gofmt -l .            # não deve listar nada
go vet ./...
go test ./...
cd installer && go vet ./... && go build ./...
```

O CI roda exatamente isso em Windows e Linux, mais o build completo do
aplicativo. Um PR que quebra o CI não é revisado até voltar ao verde.

## Convenções de código

- **Go**: `gofmt` obrigatório. Erros retornados, não engolidos — quando um erro
  é intencionalmente ignorado, um `_ =` explícito com comentário do porquê.
- **Código específico de plataforma** fica em `platform_windows.go` /
  `platform_linux.go` com build tags, nunca em `if runtime.GOOS == ...` no meio
  da lógica comum.
- **Frontend**: JavaScript puro, sem framework e sem build step além do Vite.
  Não adicione dependências de runtime ao `frontend/package.json` sem discutir
  na issue antes.
- **Comentários** explicam o *porquê*, não o *o quê*. Em português, como o resto
  do código.
- **O módulo `installer/` é Go puro.** Ele não pode ganhar nenhuma dependência
  que exija cgo, senão para de cross-compilar — que é a razão de ele existir
  como módulo separado.

## Commits e PRs

A branch `main` é protegida: **não há push direto**. Toda mudança entra por pull
request, com o CI verde. Force push e exclusão da branch estão bloqueados.

```bash
git checkout -b corrige-deteccao-de-porta
# ... suas mudanças ...
git push -u origin corrige-deteccao-de-porta
gh pr create --fill
```

O CI de um pull request vindo de fork **precisa ser liberado por um mantenedor**
antes de rodar — é o que impede alguém de fora gerar artefatos ou consumir os
runners da organização sem revisão. Não é nada contra a sua contribuição: só
abra o PR e aguarde a liberação.

- Mensagens de commit no imperativo e em português: `corrige detecção de porta
  no Linux`, não `corrigindo` nem `fixed`.
- Um PR por assunto. Refatoração e correção de bug em PRs separados.
- Descreva **como testar** a mudança: qual perfil criar, qual comando rodar, o
  que deveria acontecer. Isso vale mais que uma descrição longa do diff.
- Se a mudança altera a interface, anexe um print.

## Reportando bugs

Inclua sistema operacional e versão, versão do Exectron, o que você esperava, o
que aconteceu e — quando houver — a saída do console da aba afetada. Um passo a
passo curto de reprodução resolve a maioria dos casos.

## Licença

Ao contribuir, você concorda que sua contribuição é licenciada sob a
[MIT](LICENSE), como o resto do projeto.
