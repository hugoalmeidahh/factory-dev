//go:build webview

package main

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"runtime"
	"syscall"

	webview "github.com/webview/webview_go"
)

func init() {
	// Cocoa (macOS) e GTK (Linux) exigem que operações de UI
	// rodem na thread principal do processo (thread 0).
	// LockOSThread no init() da main goroutine garante isso.
	runtime.LockOSThread()
}

// runWebview abre uma janela nativa apontando para o servidor HTTP local.
// Deve ser chamada na goroutine principal (requisito Cocoa no macOS).
// Quando a janela é fechada, envia SIGTERM para encerrar o processo.
func runWebview(serverURL string, debug bool) {
	w := webview.New(debug)
	defer w.Destroy()

	w.SetTitle("FactoryDev")
	w.SetSize(1280, 820, webview.HintNone)
	w.SetSize(900, 600, webview.HintMin)
	w.Navigate(serverURL)

	log.Printf("Janela nativa aberta: %s", serverURL)
	w.Run() // bloqueia até fechar a janela

	// Janela fechada → encerra o processo
	fmt.Println("Janela fechada, encerrando...")
	p, _ := os.FindProcess(os.Getpid())
	_ = p.Signal(syscall.SIGTERM)
}

func hasWebview() bool { return true }

func webviewSignalNotify(quit chan os.Signal) {
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
}
