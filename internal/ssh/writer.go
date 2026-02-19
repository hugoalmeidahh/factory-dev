package ssh

import (
	"fmt"
	"os"
	"strings"

	"github.com/seuusuario/factorydev/internal/config"
	"github.com/seuusuario/factorydev/internal/storage"
)

func ApplyAccount(account storage.Account, paths *config.Paths) error {
	if err := BackupSSHConfig(paths); err != nil {
		return err
	}
	content, err := GenerateAppliedConfig(account, paths)
	if err != nil {
		return err
	}
	return writeSSHConfigAtomic(paths, content)
}

func GenerateAppliedConfig(account storage.Account, paths *config.Paths) (string, error) {
	parsed, err := ParseSSHConfig(paths.SSHConfig())
	if err != nil {
		return "", err
	}

	newBlock := buildFDevBlock(account, paths)
	updated := false
	for i := range parsed.Blocks {
		if parsed.Blocks[i].Alias == account.HostAlias && parsed.Blocks[i].IsFDev {
			parsed.Blocks[i] = SSHConfigBlock{Alias: account.HostAlias, IsFDev: true, Lines: newBlock}
			updated = true
			break
		}
	}
	if !updated {
		parsed.Blocks = append(parsed.Blocks, SSHConfigBlock{Alias: account.HostAlias, IsFDev: true, Lines: newBlock})
	}

	var out []string
	out = append(out, parsed.HeaderLines...)
	if len(out) > 0 && out[len(out)-1] != "" {
		out = append(out, "")
	}
	for idx, b := range parsed.Blocks {
		if b.IsFDev {
			out = append(out, "# BEGIN FDEV "+b.Alias)
			out = append(out, b.Lines...)
			out = append(out, "# END FDEV "+b.Alias)
		} else {
			out = append(out, b.Lines...)
		}
		if idx < len(parsed.Blocks)-1 {
			out = append(out, "")
		}
	}
	return strings.TrimSpace(strings.Join(out, "\n")) + "\n", nil
}

func buildFDevBlock(account storage.Account, paths *config.Paths) []string {
	return []string{
		"Host " + account.HostAlias,
		"  HostName " + account.HostName,
		"  User git",
		"  IdentityFile " + paths.PrivateKey(account.HostAlias),
		"  IdentitiesOnly yes",
	}
}

func writeSSHConfigAtomic(paths *config.Paths, content string) error {
	if err := os.MkdirAll(paths.SSHDir(), 0o700); err != nil {
		return fmt.Errorf("criar ~/.ssh: %w", err)
	}
	if err := os.Chmod(paths.SSHDir(), 0o700); err != nil {
		return fmt.Errorf("chmod ~/.ssh: %w", err)
	}

	tmp := paths.SSHConfig() + ".tmp"
	f, err := os.OpenFile(tmp, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return err
	}
	if _, err := f.WriteString(content); err != nil {
		_ = f.Close()
		return err
	}
	if err := f.Sync(); err != nil {
		_ = f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmp, paths.SSHConfig()); err != nil {
		return err
	}
	return os.Chmod(paths.SSHConfig(), 0o600)
}
