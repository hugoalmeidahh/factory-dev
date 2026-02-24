package git

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
)

// terminalCandidate representa um emulador de terminal e como passar o diretório de trabalho.
type terminalCandidate struct {
	bin  string
	flag string // flag para --working-directory ou equivalente
}

// OpenTerminalAt abre um emulador de terminal no diretório path.
// No macOS usa Terminal.app ou iTerm2 via osascript. No Linux detecta em ordem:
// $TERMINAL, gnome-terminal, xterm, konsole, xfce4-terminal, alacritty, kitty.
func OpenTerminalAt(path string) error {
	if runtime.GOOS == "darwin" {
		return openTerminalMacOS(path, "")
	}

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
	if runtime.GOOS == "darwin" {
		fullCmd := cmdBin
		for _, a := range args {
			fullCmd += " " + a
		}
		return openTerminalMacOS("", fullCmd)
	}

	termCandidates := []struct {
		bin     string
		execArg string
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

// openTerminalMacOS abre Terminal.app (com fallback para iTerm2) via osascript.
// Se dir for não-vazio, abre no diretório; se shellCmd for não-vazio, executa o comando.
func openTerminalMacOS(dir, shellCmd string) error {
	var script string
	if shellCmd != "" {
		script = fmt.Sprintf(`tell application "Terminal"
  activate
  do script %q
end tell`, shellCmd)
	} else {
		script = fmt.Sprintf(`tell application "Terminal"
  activate
  do script "cd %s && clear"
end tell`, dir)
	}

	cmd := exec.Command("osascript", "-e", script)
	return cmd.Start()
}
