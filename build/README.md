# build/

Recursos usados na compilação. Nada aqui vai para o repositório em forma de
binário gerado — `build/bin/` é ignorado pelo git.

| Caminho | Para que serve |
| --- | --- |
| `appicon.png` | Ícone base do aplicativo (512x512). Também é o ícone da bandeja no Linux, embutido por `icon_linux.go`. |
| `windows/icon.ico` | Ícone do executável e da bandeja no Windows, embutido por `icon_windows.go`. Multi-resolução, de 16 a 256 px. |
| `windows/info.json` | Metadados de versão gravados no `.exe` pelo Wails. |
| `windows/wails.exe.manifest` | Manifesto do executável: common controls e DPI por monitor. |
| `linux/icon-256.png` | Ícone que o instalador grava junto do `.desktop`. |
| `linux/Dockerfile.build` | Imagem de compilação do binário Linux, usada por `scripts/package.sh`. |
| `bin/` | Saída do `wails build`. Ignorado pelo git. |

Para trocar o ícone do aplicativo, substitua `appicon.png` por um PNG quadrado
de 512x512 e regenere `windows/icon.ico` e `linux/icon-256.png` a partir dele.
