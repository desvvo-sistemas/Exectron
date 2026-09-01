// Modulo separado de proposito: o instalador nao pode arrastar as dependencias
// do aplicativo (Wails, systray e cgo). Assim ele e Go puro e cross-compila
// para Windows e Linux a partir de qualquer maquina.
module exectron-installer

go 1.24
