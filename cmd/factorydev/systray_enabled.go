//go:build systray

package main

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"log"
	"os"
	"os/exec"
	"runtime"
	"syscall"

	"github.com/getlantern/systray"
)

func runSystray(serverURL string) {
	systray.Run(onSystrayReady(serverURL), func() {})
}

func onSystrayReady(serverURL string) func() {
	return func() {
		systray.SetIcon(iconBytes())
		systray.SetTooltip("FactoryDev")

		mOpen := systray.AddMenuItem("Abrir FactoryDev", serverURL)
		systray.AddSeparator()
		mQuit := systray.AddMenuItem("Sair", "Encerrar FactoryDev")

		go func() {
			for {
				select {
				case <-mOpen.ClickedCh:
					openURL(serverURL)
				case <-mQuit.ClickedCh:
					systray.Quit()
					if err := syscall.Kill(os.Getpid(), syscall.SIGTERM); err != nil {
						log.Printf("systray: erro ao encerrar: %v", err)
						os.Exit(0)
					}
				}
			}
		}()
	}
}

func openURL(url string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "linux":
		cmd = exec.Command("xdg-open", url)
	case "darwin":
		cmd = exec.Command("open", url)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	default:
		return
	}
	_ = cmd.Start()
}

// iconBytes gera um ícone 32×32 PNG em memória (círculo verde da marca).
func iconBytes() []byte {
	const size = 32
	img := image.NewRGBA(image.Rect(0, 0, size, size))
	accent := color.RGBA{R: 0x0e, G: 0x6d, B: 0x5f, A: 0xff}
	cx, cy := float64(size)/2, float64(size)/2
	r := float64(size)/2 - 1
	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			dx := float64(x) + 0.5 - cx
			dy := float64(y) + 0.5 - cy
			if dx*dx+dy*dy <= r*r {
				img.Set(x, y, accent)
			}
		}
	}
	var buf bytes.Buffer
	_ = png.Encode(&buf, img)
	return buf.Bytes()
}
