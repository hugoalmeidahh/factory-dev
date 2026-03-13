//go:build !webview

package main

import (
	"os"
	"os/signal"
	"syscall"
)

func runWebview(_ string, _ bool) {
	// webview não compilado; use: go build -tags webview
}

func hasWebview() bool { return false }

func webviewSignalNotify(quit chan os.Signal) {
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
}
