package config

import (
	"flag"
	"fmt"
	"log"
)

type Config struct {
	Host        string
	Port        int
	Debug       bool
	OpenBrowser bool
	WindowMode  string
	FDevDir     string
}

func ParseFlags(args []string) *Config {
	fs := flag.NewFlagSet("factorydev", flag.ExitOnError)
	cfg := &Config{}
	fs.StringVar(&cfg.Host, "host", "127.0.0.1", "Endereço de escuta")
	fs.IntVar(&cfg.Port, "port", 7331, "Porta HTTP (1024-65535)")
	fs.BoolVar(&cfg.Debug, "debug", false, "Modo debug")
	fs.BoolVar(&cfg.OpenBrowser, "open-browser", true, "Abre navegador automaticamente ao iniciar o servidor")
	fs.StringVar(&cfg.WindowMode, "window-mode", "app", "Modo de abertura: app|browser")
	_ = fs.Parse(args)

	if cfg.Port < 1024 || cfg.Port > 65535 {
		log.Fatalf("porta inválida: %d", cfg.Port)
	}
	if cfg.WindowMode != "app" && cfg.WindowMode != "browser" {
		log.Fatalf("window-mode inválido: %s (use app ou browser)", cfg.WindowMode)
	}

	return cfg
}

func (c *Config) Addr() string {
	return fmt.Sprintf("%s:%d", c.Host, c.Port)
}
