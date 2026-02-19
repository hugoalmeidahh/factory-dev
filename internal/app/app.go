package app

import (
	"fmt"
	"log/slog"

	"github.com/seuusuario/factorydev/internal/config"
	"github.com/seuusuario/factorydev/internal/git"
	"github.com/seuusuario/factorydev/internal/storage"
)

type App struct {
	Config     *config.Config
	Storage    storage.Storage
	Logger     *slog.Logger
	Paths      *config.Paths
	SSHService *SSHService
	GitService *git.Service
}

type SSHService struct{}

func New(cfg *config.Config) (*App, error) {
	paths, err := config.NewPaths()
	if err != nil {
		return nil, fmt.Errorf("resolver paths: %w", err)
	}
	cfg.FDevDir = paths.Base

	if err := config.EnsureDirectories(paths); err != nil {
		return nil, fmt.Errorf("garantir diretórios: %w", err)
	}

	logger, err := NewLogger(paths, cfg.Debug)
	if err != nil {
		return nil, fmt.Errorf("inicializar logger: %w", err)
	}

	st := storage.NewJSONStorage(paths.State)
	if _, err := st.LoadState(); err != nil {
		return nil, fmt.Errorf("carregar state inicial: %w", err)
	}

	return &App{
		Config:     cfg,
		Storage:    st,
		Logger:     logger,
		Paths:      paths,
		SSHService: &SSHService{},
		GitService: git.NewService(),
	}, nil
}
