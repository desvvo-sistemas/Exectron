<div align="center">

# Exectron

**Runner local de projetos para quem roda vários serviços ao mesmo tempo.**

Sobe todos os processos do seu ambiente de uma vez, acompanha cada console por
abas, gerencia versões do Node, edita `appsettings.json` em árvore e controla
containers Docker — tudo numa janela só.

[![Licença: MIT](https://img.shields.io/badge/licen%C3%A7a-MIT-blue.svg)](LICENSE)
[![Go](https://img.shields.io/badge/Go-1.23%2B-00ADD8?logo=go&logoColor=white)](https://go.dev)
[![Wails](https://img.shields.io/badge/Wails-v2-DF0000)](https://wails.io)
[![Plataformas](https://img.shields.io/badge/plataformas-Windows%20%7C%20Linux-lightgrey)](#instalação)

</div>

---

## O que ele faz

| Recurso | Descrição |
| --- | --- |
| **Perfis de projeto** | Cada serviço vira um perfil com pasta, comando, variáveis de ambiente e porta. Um clique sobe, outro derruba. |
| **Consoles em abas** | Saída de cada processo em sua própria aba, com detecção automática de porta e link para a documentação. |
| **Gerenciador de Node** | Instala, ativa e remove versões do Node sem depender de nvm. |
| **Editor de `appsettings.json`** | Navegação em árvore com edição direta — útil em soluções .NET com vários projetos. |
| **Docker opcional** | Aba para containers, imagens e `docker compose`. Sem a CLI instalada, o resto do app segue normal. |
| **Bandeja do sistema** | A janela fecha para a bandeja e os processos continuam rodando. |

Presets prontos para **Node** (`npm`/`pnpm`/`yarn`), **.NET** (`dotnet run`/`watch`),
**Go** (`go run`) e **Python** (`uvicorn`/`flask`).

## Instalação

Baixe o instalador da sua plataforma em
[Releases](https://github.com/desvvo-sistemas/Exectron/releases):

| Arquivo | Sistema |
| --- | --- |
| `exectron-setup-windows-amd64.exe` | Windows 10/11 x64 |
| `exectron-setup-linux-amd64` | Linux x64 |

O instalador é um executável único, com o aplicativo embutido. Ao rodar ele:

1. instala o aplicativo no perfil do usuário, **sem exigir administrador**
   (`%LOCALAPPDATA%\Programs\Exectron` ou `~/.local/share/exectron`);
2. cria a base de perfis **vazia** em `%APPDATA%\exectron` ou `~/.config/exectron`
   — os perfis são criados por você dentro do aplicativo;
3. registra atalho no menu, entrada de desinstalação e o comando no PATH;
4. provisiona o **toolchain Go** se a máquina não tiver;
5. provisiona o **Node LTS** se a máquina não tiver, já marcado como versão ativa.

Os passos 4 e 5 baixam de `go.dev` e `nodejs.org`, então a primeira execução
precisa de rede. Uma instalação existente de Go ou Node é detectada e respeitada.

```bash
chmod +x exectron-setup-linux-amd64 && ./exectron-setup-linux-amd64  # Linux, interativo
exectron-setup-windows-amd64.exe --silent                            # Windows, sem perguntas
```

Opções: `--dir <pasta>`, `--silent`, `--skip-go`, `--skip-node`, `--uninstall`.

No Linux o aplicativo depende de **GTK3**, **WebKit2GTK** e
**libayatana-appindicator3** — presentes na maioria das distribuições desktop.
Em uma instalação enxuta:

```bash
# Debian/Ubuntu
sudo apt install libgtk-3-0 libwebkit2gtk-4.1-0 libayatana-appindicator3-1
# Fedora
sudo dnf install gtk3 webkit2gtk4.1 libayatana-appindicator-gtk3
```

Para desinstalar use "Aplicativos instalados" no Windows, ou rode o
`exectron-uninstall` que fica na pasta de instalação. Os perfis não são apagados.

> O Docker **não** é requisito do aplicativo. Sem a CLI instalada, a aba Docker
> apenas informa isso e o resto segue funcionando normalmente.

## Gerando os instaladores

```bash
./scripts/package.sh windows   # dist/exectron-setup-windows-amd64.exe
./scripts/package.sh linux     # dist/exectron-setup-linux-amd64
./scripts/package.sh all       # os dois
```

Para o alvo Windows basta ter Go, Node e a CLI do Wails.

O binário **Linux** do aplicativo precisa de cgo com GTK3, WebKit2 e
libayatana-appindicator, o que não existe numa máquina Windows. Por isso ele é
compilado dentro de um container definido em `build/linux/Dockerfile.build`, e o
script cuida disso sozinho. O Docker aqui é ferramenta de compilação: não entra
no instalador nem no aplicativo. Rodando o script em uma máquina Linux que já
tenha essas bibliotecas, dá para trocar a chamada do container por um
`wails build -platform linux/amd64` direto.

Os mesmos artefatos saem do CI:

- [`ci.yml`](.github/workflows/ci.yml) roda a cada push e pull request. Além de
  `gofmt`, `go vet` e `go test`, ele monta os dois instaladores e os guarda como
  artefato do build por 14 dias — dá para baixar de qualquer PR.
- [`release.yml`](.github/workflows/release.yml) roda quando uma tag `v*` é
  publicada: compila os dois instaladores, gera o `SHA256SUMS.txt` e anexa tudo
  à [Release](https://github.com/desvvo-sistemas/Exectron/releases).

```bash
git tag v1.0.0 && git push origin v1.0.0   # publica a release com os instaladores
```

## Desenvolvimento

```bash
wails dev        # hot reload; também serve em http://localhost:34115
wails build      # binário de produção em build/bin
go test ./...    # testes do backend
```

Pré-requisitos: [Go 1.23+](https://go.dev/dl/), [Node 18+](https://nodejs.org) e a
[CLI do Wails](https://wails.io/docs/gettingstarted/installation)
(`go install github.com/wailsapp/wails/v2/cmd/wails@latest`).

### Organização

| Caminho | O que tem |
| --- | --- |
| `app.go` | Ciclo de vida, perfis, execução de processos, gerenciador de Node |
| `docker.go` | Perfis docker, containers, imagens e compose |
| `settings_tree.go` | Leitura e escrita do `appsettings.json` em árvore |
| `platform_windows.go` / `platform_linux.go` | Shell, árvore de processos, pacote do Node e troca de versão |
| `icon_windows.go` / `icon_linux.go` | Ícone da bandeja (`.ico` no Windows, `.png` no Linux) |
| `frontend/src/` | Interface (JS puro + CSS, sem framework) |
| `installer/` | Módulo separado do instalador; Go puro, cross-compila sozinho |
| `build/` | Ícones do aplicativo e Dockerfile de compilação para Linux |
| `scripts/package.sh` | Monta os instaladores |

O `installer/` é um módulo Go próprio de propósito: assim ele não arrasta as
dependências do aplicativo (Wails, systray, cgo) e cross-compila a partir de
qualquer máquina.

### Dados no disco

| Caminho | Conteúdo |
| --- | --- |
| `<config>/exectron/projects.json` | Perfis salvos |
| `<config>/exectron/settings.json` | Versão ativa do Node e cache de versões |
| `<config>/exectron/node/<versão>` | Versões do Node gerenciadas pelo aplicativo |

`<config>` é `%APPDATA%` no Windows e `~/.config` no Linux. Instalações antigas
que usavam a pasta `starter-project` são migradas na primeira execução; as versões
do Node já instaladas continuam onde estão, porque o PATH do sistema aponta para lá.

## Contribuindo

Issues e pull requests são bem-vindos — veja [CONTRIBUTING.md](CONTRIBUTING.md)
para o fluxo e as convenções, e o [Código de Conduta](CODE_OF_CONDUCT.md).

## Licença

[MIT](LICENSE) © desvvo-sistemas
