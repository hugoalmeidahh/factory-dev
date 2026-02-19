package config

import (
	"fmt"
	"io/fs"
	"os"
)

func EnsureDirectories(paths *Paths) error {
	dirs := []struct {
		path string
		perm fs.FileMode
	}{
		{paths.Base, 0o700},
		{paths.Keys, 0o700},
		{paths.Logs, 0o755},
		{paths.Backups, 0o755},
	}

	for _, d := range dirs {
		if err := os.MkdirAll(d.path, d.perm); err != nil {
			return fmt.Errorf("criar diretório %s: %w", d.path, err)
		}
		if err := os.Chmod(d.path, d.perm); err != nil {
			return fmt.Errorf("ajustar permissão %s: %w", d.path, err)
		}
	}

	return nil
}
