package git

import (
	"fmt"
	"os"
	"os/exec"
)

// terminalCandidate representa um emulador de terminal e como passar o diretório de trabalho.
type terminalCandidate struct {
	bin  string
	flag string // flag para --working-directory ou equivalente
}

// OpenTerminalAt abre um emulador de terminal no diretório path.
// Detecta o terminal em ordem: $TERMINAL, gnome-terminal, xterm, konsole,
// xfce4-terminal, alacritty, kitty.
func OpenTerminalAt(path string) error {
	candidates := []terminalCandidate{
		{os.Getenv("TERMINAL"), "--working-directory"},
		{"gnome-terminal", "--working-directory"},
		{"xfce4-terminal", "--working-directory"},
		{"konsole", "--workdir"},
		{"alacritty", "--working-directory"},
		{"kitty", "-d"},
		{"xterm", "-e"},
	}

	for _, c := range candidates {
		if c.bin == "" {
			continue
		}
		binPath, err := exec.LookPath(c.bin)
		if err != nil {
			continue
		}
		var cmd *exec.Cmd
		if c.bin == "xterm" {
			// xterm não suporta --working-directory; usa bash com cd
			cmd = exec.Command(binPath, "-e", "bash", "-c", "cd "+path+" && exec bash")
		} else {
			cmd = exec.Command(binPath, c.flag, path)
		}
		cmd.Env = os.Environ()
		return cmd.Start()
	}

	return fmt.Errorf("nenhum terminal encontrado; defina a variável $TERMINAL (ex: export TERMINAL=gnome-terminal)")
}

// OpenTerminalWithCmd abre um terminal executando um comando específico (ex: ssh user@host).
func OpenTerminalWithCmd(cmdBin string, args ...string) error {
	termCandidates := []struct {
		bin     string
		execArg string // flag para passar comando a executar
	}{
		{os.Getenv("TERMINAL"), "-e"},
		{"gnome-terminal", "--"},
		{"xfce4-terminal", "-e"},
		{"konsole", "-e"},
		{"alacritty", "-e"},
		{"kitty", ""},
		{"xterm", "-e"},
	}

	cmdArgs := append([]string{cmdBin}, args...)

	for _, c := range termCandidates {
		if c.bin == "" {
			continue
		}
		binPath, err := exec.LookPath(c.bin)
		if err != nil {
			continue
		}
		var termArgs []string
		if c.execArg != "" {
			termArgs = append([]string{c.execArg}, cmdArgs...)
		} else {
			termArgs = cmdArgs
		}
		cmd := exec.Command(binPath, termArgs...)
		cmd.Env = os.Environ()
		return cmd.Start()
	}
	return fmt.Errorf("nenhum terminal encontrado; defina a variável $TERMINAL")
}
