//go:build linux

package main

import _ "embed"

// A bandeja do Linux espera PNG; o .ico do Windows nao e renderizado.
//
//go:embed build/appicon.png
var appIcon []byte
