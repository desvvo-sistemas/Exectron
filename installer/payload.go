package main

import _ "embed"

// O conteudo de payload/ e gerado pelo script de build: o binario do aplicativo
// comprimido e o icone. Ficam embutidos para que o instalador seja um arquivo so.

//go:embed payload/app.gz
var appPayload []byte

//go:embed payload/icon.png
var iconPayload []byte
