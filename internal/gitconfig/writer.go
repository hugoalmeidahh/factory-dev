package gitconfig

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// SetGlobalValue define um valor no gitconfig global via git config --global.
func SetGlobalValue(key, value string) error {
	cmd := exec.Command("git", "config", "--global", key, value)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git config --global %s: %s", key, strings.TrimSpace(string(out)))
	}
	return nil
}

// AddIncludeIf acrescenta uma regra [includeIf] ao arquivo globalPath de forma atômica.
func AddIncludeIf(globalPath string, rule IncludeIfRule) error {
	// Garante que não existe duplicata
	existing, err := ListIncludeIf(globalPath)
	if err != nil {
		return err
	}
	for _, r := range existing {
		if r.Pattern == rule.Pattern {
			return fmt.Errorf("regra para padrão '%s' já existe", rule.Pattern)
		}
	}

	f, err := os.OpenFile(globalPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("abrir gitconfig: %w", err)
	}
	defer f.Close()

	block := fmt.Sprintf("\n[includeIf %q]\n\tpath = %s\n", rule.Pattern, rule.IncludePath)
	if _, err := f.WriteString(block); err != nil {
		return fmt.Errorf("escrever includeIf: %w", err)
	}
	return nil
}

// RemoveIncludeIf remove a regra [includeIf] com o padrão dado reescrevendo o arquivo.
func RemoveIncludeIf(globalPath string, pattern string) error {
	f, err := os.Open(globalPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	var lines []string
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	f.Close()
	if err := scanner.Err(); err != nil {
		return err
	}

	var out []string
	skip := false
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "[") {
			end := strings.Index(trimmed, "]")
			if end > 0 {
				header := trimmed[1:end]
				lower := strings.ToLower(header)
				if strings.HasPrefix(lower, "includeif ") {
					q1 := strings.Index(header, `"`)
					q2 := strings.LastIndex(header, `"`)
					if q1 >= 0 && q2 > q1 && header[q1+1:q2] == pattern {
						skip = true
						continue
					}
				}
			}
			skip = false
		}
		if !skip {
			out = append(out, line)
		}
	}

	tmp, err := os.CreateTemp("", "gitconfig-*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())

	for _, l := range out {
		fmt.Fprintln(tmp, l)
	}
	tmp.Close()

	return os.Rename(tmp.Name(), globalPath)
}
