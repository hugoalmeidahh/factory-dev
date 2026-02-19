package doctor

import (
	"os"
	"os/exec"
	"path/filepath"

	"github.com/seuusuario/factorydev/internal/config"
)

type Check struct {
	Name    string
	OK      bool
	Message string
}

func RunDoctor(paths *config.Paths) []Check {
	checks := make([]Check, 0, 4)

	sshDir := paths.SSHDir()
	st, err := os.Stat(sshDir)
	checks = append(checks, Check{
		Name:    "~/.ssh/ existe",
		OK:      err == nil,
		Message: ifElse(err == nil, "OK", "Crie com: mkdir -m 700 ~/.ssh"),
	})

	permOK := false
	if err == nil && st.IsDir() {
		permOK = st.Mode().Perm() == 0o700
	}
	checks = append(checks, Check{
		Name:    "~/.ssh/ permissão 0700",
		OK:      permOK,
		Message: ifElse(permOK, "OK", "Ajuste com: chmod 700 ~/.ssh"),
	})

	_, err = exec.LookPath("ssh")
	checks = append(checks, Check{
		Name:    "ssh disponível no PATH",
		OK:      err == nil,
		Message: ifElse(err == nil, "OK", "Instale o OpenSSH"),
	})

	_, err = os.Stat(paths.Base)
	checks = append(checks, Check{
		Name:    filepath.Base(paths.Base) + " criado",
		OK:      err == nil,
		Message: ifElse(err == nil, "OK", "Diretório base não encontrado"),
	})

	return checks
}

func ifElse(ok bool, a, b string) string {
	if ok {
		return a
	}
	return b
}
