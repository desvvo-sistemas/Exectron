# Exectron

Runner local de projetos: sobe varios processos ao mesmo tempo, acompanha cada
console por abas, gerencia versoes do Node, edita `appsettings.json` em arvore e
sobe containers Docker.

Aplicacao desktop em [Wails](https://wails.io) (Go + WebView), com frontend em
JavaScript sem framework.

## Instaladores

Os artefatos ficam em `dist/`:

| Arquivo | Sistema |
| --- | --- |
| `exectron-setup-windows-amd64.exe` | Windows 10/11 x64 |
| `exectron-setup-linux-amd64` | Linux x64 |

O instalador e um executavel unico, com o aplicativo embutido. Ao rodar ele:

1. instala o aplicativo no perfil do usuario, **sem exigir administrador**
   (`%LOCALAPPDATA%\Programs\Exectron` ou `~/.local/share/exectron`);
2. cria a base de perfis **vazia** em `%APPDATA%\exectron` ou `~/.config/exectron`
   — os perfis sao criados por voce dentro do aplicativo;
3. registra atalho no menu, entrada de desinstalacao e o comando no PATH;
4. provisiona o **toolchain Go** se a maquina nao tiver;
5. provisiona o **Node LTS** se a maquina nao tiver, ja marcado como versao ativa.

Os passos 4 e 5 baixam de `go.dev` e `nodejs.org`, entao a primeira execucao
precisa de rede. Uma instalacao existente de Go ou Node e detectada e respeitada.

```
exectron-setup-linux-amd64                 # instalacao interativa
exectron-setup-windows-amd64.exe --silent  # sem confirmacao nem pausa
```

Opcoes: `--dir <pasta>`, `--silent`, `--skip-go`, `--skip-node`, `--uninstall`.

Para desinstalar use "Aplicativos instalados" no Windows, ou rode o
`exectron-uninstall` que fica na pasta de instalacao. Os perfis nao sao apagados.

> O Docker **nao** e requisito do aplicativo. Sem a CLI instalada, a aba Docker
> apenas informa isso e o resto segue funcionando normalmente.

## Gerando os instaladores

```bash
./scripts/package.sh windows   # instalador Windows
./scripts/package.sh linux     # instalador Linux
./scripts/package.sh all       # os dois
```

Para o alvo Windows basta ter Go, Node e a CLI do Wails.

O binario **Linux** do aplicativo precisa de cgo com GTK3, WebKit2 e
libayatana-appindicator, o que nao existe numa maquina Windows. Por isso ele e
compilado dentro de um container definido em `build/linux/Dockerfile.build`, e o
script cuida disso sozinho. O Docker aqui e ferramenta de compilacao: nao entra
no instalador nem no aplicativo. Rodando o script em uma maquina Linux que ja
tenha essas bibliotecas, da para trocar a chamada do container por um
`wails build -platform linux/amd64` direto.

## Desenvolvimento

```bash
wails dev        # hot reload; tambem serve em http://localhost:34115
wails build      # binario de producao em build/bin
go test ./...    # testes do backend
```

### Organizacao

| Caminho | O que tem |
| --- | --- |
| `app.go` | Ciclo de vida, perfis, execucao de processos, gerenciador de Node |
| `docker.go` | Perfis docker, containers, imagens e compose |
| `settings_tree.go` | Leitura e escrita do `appsettings.json` em arvore |
| `platform_windows.go` / `platform_linux.go` | Shell, arvore de processos, pacote do Node e troca de versao |
| `icon_windows.go` / `icon_linux.go` | Icone da bandeja (`.ico` no Windows, `.png` no Linux) |
| `frontend/src/` | Interface (JS puro + CSS) |
| `installer/` | Modulo separado do instalador; Go puro, cross-compila sozinho |
| `scripts/package.sh` | Monta os instaladores |

O `installer/` e um modulo Go proprio de proposito: assim ele nao arrasta as
dependencias do aplicativo (Wails, systray, cgo) e cross-compila a partir de
qualquer maquina.

### Dados no disco

| Caminho | Conteudo |
| --- | --- |
| `<config>/exectron/projects.json` | Perfis salvos |
| `<config>/exectron/settings.json` | Versao ativa do Node e cache de versoes |
| `<config>/exectron/node/<versao>` | Versoes do Node gerenciadas pelo aplicativo |

`<config>` e `%APPDATA%` no Windows e `~/.config` no Linux. Instalacoes antigas
que usavam a pasta `starter-project` sao migradas na primeira execucao; as versoes
do Node ja instaladas continuam onde estao, porque o PATH do sistema aponta para la.
