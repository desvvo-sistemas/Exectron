#!/usr/bin/env bash
#
# Monta os instaladores do Exectron.
#
#   ./scripts/package.sh windows   -> dist/exectron-setup-windows-amd64.exe
#   ./scripts/package.sh linux     -> dist/exectron-setup-linux-amd64
#   ./scripts/package.sh all       -> os dois
#
# O instalador e um executavel unico com o aplicativo embutido. Na maquina de
# destino ele instala o app, cria a base de perfis vazia, registra atalhos e
# provisiona Go e Node. Nao exige administrador nem Docker.
#
# O binario Linux do aplicativo precisa de cgo com GTK3/WebKit2, entao sai de um
# container (build/linux/Dockerfile.build). O Docker e ferramenta de compilacao:
# nao e dependencia do instalador nem do aplicativo.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
DIST="$ROOT/dist"
PAYLOAD="$ROOT/installer/payload"
IMAGE="exectron-linux-build:latest"
GOVERSIONINFO="github.com/josephspurrier/goversioninfo/cmd/goversioninfo@v1.4.1"

cd "$ROOT"
mkdir -p "$DIST" "$PAYLOAD"

log() { printf '\n==> %s\n' "$*"; }

build_app_windows() {
  log "Compilando o aplicativo para Windows"
  wails build -platform windows/amd64
  cp "build/bin/exectron.exe" "$PAYLOAD/app"
}

# host_path devolve o caminho no formato que o Docker Desktop espera. No Git
# Bash os caminhos sao POSIX e precisam voltar para a forma do Windows.
host_path() {
  if command -v cygpath >/dev/null 2>&1; then cygpath -w "$1"; else printf '%s' "$1"; fi
}

build_app_linux() {
  log "Compilando o aplicativo para Linux (container)"
  if ! docker image inspect "$IMAGE" >/dev/null 2>&1; then
    log "Construindo a imagem de compilacao $IMAGE"
    docker build -f build/linux/Dockerfile.build -t "$IMAGE" build/linux
  fi

  # A fonte entra somente leitura e a compilacao acontece numa copia dentro do
  # container: assim o node_modules do Linux nao substitui o do host, que tem
  # binarios nativos (esbuild) e quebraria o build seguinte de Windows.
  # MSYS_NO_PATHCONV impede o Git Bash de reescrever os caminhos do container.
  MSYS_NO_PATHCONV=1 MSYS2_ARG_CONV_EXCL='*' docker run --rm \
    -v "$(host_path "$ROOT"):/src:ro" \
    -v "$(host_path "$DIST"):/out" \
    -v exectron-gomod:/go/pkg/mod \
    "$IMAGE" \
    bash -c 'set -e
      mkdir -p /work
      tar -C /src -cf - \
        --exclude=./frontend/node_modules --exclude=./build/bin \
        --exclude=./dist --exclude=./.git --exclude=./installer/payload \
        --exclude="*.syso" --exclude="*.exe" . | tar -C /work -xf -
      cd /work
      wails build -platform linux/amd64 -o exectron
      cp build/bin/exectron /out/exectron-linux-amd64'

  cp "$DIST/exectron-linux-amd64" "$PAYLOAD/app"
}

# windows_resources gera o .syso com icone e metadados de versao do instalador.
# Sem isso o setup.exe sai com o icone generico do Windows. O sufixo _windows no
# nome do arquivo faz o Go linkar o recurso apenas no alvo Windows.
windows_resources() {
  log "Gerando o icone e os metadados do instalador"
  ( cd installer && go run "$GOVERSIONINFO" -o resource_windows.syso versioninfo.json )
}

pack() {
  local goos="$1" out="$2"
  log "Empacotando o instalador para $goos"
  gzip -9 -c "$PAYLOAD/app" > "$PAYLOAD/app.gz"
  rm -f "$PAYLOAD/app"
  cp "build/linux/icon-256.png" "$PAYLOAD/icon.png"

  if [ "$goos" = "windows" ]; then windows_resources; fi

  ( cd installer && GOOS="$goos" GOARCH=amd64 CGO_ENABLED=0 \
      go build -trimpath -ldflags "-s -w" -o "$DIST/$out" . )
  printf '    %s (%s)\n' "$DIST/$out" "$(du -h "$DIST/$out" | cut -f1)"
}

target="${1:-all}"

case "$target" in
  windows)
    build_app_windows
    pack windows "exectron-setup-windows-amd64.exe"
    ;;
  linux)
    build_app_linux
    pack linux "exectron-setup-linux-amd64"
    ;;
  all)
    build_app_windows
    pack windows "exectron-setup-windows-amd64.exe"
    build_app_linux
    pack linux "exectron-setup-linux-amd64"
    ;;
  *)
    echo "uso: $0 [windows|linux|all]" >&2
    exit 1
    ;;
esac

log "Pronto"
ls -la "$DIST"
