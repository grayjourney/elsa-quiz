// Package web holds the embedded single-file frontend served by the quiz server.
package web

import _ "embed"

// Index is the compiled-in web client (web/index.html), served at GET /.
//
//go:embed index.html
var Index []byte
