package ssh

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/seuusuario/factorydev/internal/config"
)

func BackupSSHConfig(paths *config.Paths) error {
	src := paths.SSHConfig()
	if _, err := os.Stat(src); os.IsNotExist(err) {
		return nil
	}
	ts := time.Now().Format("20060102_150405")
	dst := filepath.Join(paths.Backups, "ssh_config_"+ts)
	if err := copyFile(src, dst, 0o600); err != nil {
		return fmt.Errorf("backup ssh_config: %w", err)
	}
	slog.Info("backup criado", "path", dst)
	return pruneOldBackups(paths.Backups, "ssh_config_", 10)
}

func copyFile(src, dst string, perm os.FileMode) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, perm)
	if err != nil {
		return err
	}
	defer out.Close()

	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	if err := out.Sync(); err != nil {
		return err
	}
	return os.Chmod(dst, perm)
}

func pruneOldBackups(dir, prefix string, keep int) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	var names []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if strings.HasPrefix(name, prefix) {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	if len(names) <= keep {
		return nil
	}
	for _, name := range names[:len(names)-keep] {
		if err := os.Remove(filepath.Join(dir, name)); err != nil {
			return err
		}
	}
	return nil
}
