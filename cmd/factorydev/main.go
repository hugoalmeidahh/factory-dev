package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"runtime"
	"syscall"
	"time"

	"github.com/seuusuario/factorydev/internal/app"
	"github.com/seuusuario/factorydev/internal/config"
	"github.com/seuusuario/factorydev/internal/doctor"
	"github.com/seuusuario/factorydev/internal/handler"
)

var Version = "dev"

func main() {
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "doctor":
			runDoctorCLI()
			return
		case "version":
			fmt.Printf("FactoryDev %s\n", Version)
			return
		}
	}
	runServer()
}

func runServer() {
	cfg := config.ParseFlags(os.Args[1:])
	log.Printf("FactoryDev iniciando em %s", cfg.Addr())

	a, err := app.New(cfg)
	if err != nil {
		log.Fatal(err)
	}

	h := handler.New(a)
	srv := &http.Server{
		Addr:    cfg.Addr(),
		Handler: h.Routes(),
	}

	go func() {
		a.Logger.Info("servidor iniciado", "addr", cfg.Addr())
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal(err)
		}
	}()
	if cfg.OpenBrowser {
		go openBrowser(a, cfg)
	}

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		a.Logger.Error("erro no shutdown", "err", err)
	}
}

func openBrowser(a *app.App, cfg *config.Config) {
	time.Sleep(250 * time.Millisecond)
	host := cfg.Host
	if host == "0.0.0.0" || host == "" {
		host = "127.0.0.1"
	}
	url := fmt.Sprintf("http://%s:%d/", host, cfg.Port)

	if runtime.GOOS == "linux" {
		if os.Getenv("DISPLAY") == "" && os.Getenv("WAYLAND_DISPLAY") == "" {
			a.Logger.Warn("sem sessão gráfica; não foi possível abrir navegador automaticamente", "url", url)
			log.Printf("Abra manualmente: %s", url)
			return
		}
	}

	var candidates [][]string
	switch runtime.GOOS {
	case "linux":
		candidates = append(candidates,
			[]string{"kde-open5", url},
			[]string{"kde-open", url},
		)
		if cfg.WindowMode == "app" {
			candidates = append(candidates,
				[]string{"google-chrome", "--new-window", "--app=" + url},
				[]string{"google-chrome-stable", "--new-window", "--app=" + url},
				[]string{"chromium", "--new-window", "--app=" + url},
				[]string{"chromium-browser", "--new-window", "--app=" + url},
				[]string{"microsoft-edge", "--new-window", "--app=" + url},
				[]string{"brave-browser", "--new-window", "--app=" + url},
			)
		}
		candidates = append(candidates,
			[]string{"xdg-open", url},
			[]string{"gio", "open", url},
			[]string{"sensible-browser", url},
			[]string{"x-www-browser", url},
		)
	case "darwin":
		if cfg.WindowMode == "app" {
			candidates = append(candidates, []string{"open", "-na", "Google Chrome", "--args", "--new-window", "--app=" + url})
		}
		candidates = append(candidates, []string{"open", url})
	case "windows":
		if cfg.WindowMode == "app" {
			candidates = append(candidates,
				[]string{"cmd", "/c", "start", "msedge", "--new-window", "--app=" + url},
				[]string{"cmd", "/c", "start", "chrome", "--new-window", "--app=" + url},
			)
		}
		candidates = append(candidates, []string{"rundll32", "url.dll,FileProtocolHandler", url})
	default:
		a.Logger.Info("sistema não suportado para auto-open", "os", runtime.GOOS)
		log.Printf("Abra manualmente: %s", url)
		return
	}

	var openErr error
	for _, c := range candidates {
		if _, err := exec.LookPath(c[0]); err != nil {
			openErr = errors.Join(openErr, err)
			continue
		}

		a.Logger.Debug("tentando abrir navegador", "cmd", c[0], "args", c[1:])

		cmd := exec.Command(c[0], c[1:]...)
		out, err := cmd.CombinedOutput()
		if err != nil {
			if len(out) > 0 {
				a.Logger.Warn("falha ao abrir com comando", "cmd", c[0], "err", err, "output", string(out))
			} else {
				a.Logger.Warn("falha ao abrir com comando", "cmd", c[0], "err", err)
			}
			openErr = errors.Join(openErr, err)
			continue
		}
		a.Logger.Info("navegador aberto automaticamente", "url", url, "cmd", c[0])
		return
	}
	a.Logger.Warn("falha ao abrir navegador automaticamente", "err", openErr, "url", url)
	log.Printf("Abra manualmente: %s", url)
}

func runDoctorCLI() {
	paths, err := config.NewPaths()
	if err != nil {
		log.Fatal(err)
	}
	_ = config.EnsureDirectories(paths)
	checks := doctor.RunDoctor(paths)
	allOK := true
	for _, c := range checks {
		status := "✓"
		if !c.OK {
			status = "✗"
			allOK = false
		}
		fmt.Printf("%s  %s: %s\n", status, c.Name, c.Message)
	}
	if !allOK {
		os.Exit(1)
	}
}
