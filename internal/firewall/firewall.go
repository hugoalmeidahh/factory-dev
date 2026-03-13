package firewall

import (
	"fmt"
	"os/exec"
	"runtime"
	"strings"
)

// Status representa o estado do firewall.
type Status struct {
	Engine  string // "ufw", "firewalld", "pf", "none"
	Enabled bool
	Rules   []Rule
}

// Rule representa uma regra de firewall.
type Rule struct {
	ID     string
	Port   string
	Proto  string
	Action string // "allow" ou "deny"
	Raw    string
}

// Detect retorna o engine de firewall disponível.
func Detect() string {
	if runtime.GOOS == "darwin" {
		if _, err := exec.LookPath("/usr/libexec/ApplicationFirewall/socketfilterfw"); err == nil {
			return "pf"
		}
		return "none"
	}
	if _, err := exec.LookPath("ufw"); err == nil {
		return "ufw"
	}
	if _, err := exec.LookPath("firewall-cmd"); err == nil {
		return "firewalld"
	}
	return "none"
}

// GetStatus retorna o estado atual do firewall.
func GetStatus() Status {
	engine := Detect()
	s := Status{Engine: engine}

	switch engine {
	case "pf":
		out, err := exec.Command("/usr/libexec/ApplicationFirewall/socketfilterfw", "--getglobalstate").CombinedOutput()
		if err == nil {
			s.Enabled = strings.Contains(string(out), "enabled")
		}
	case "ufw":
		out, err := exec.Command("sudo", "ufw", "status").CombinedOutput()
		if err == nil {
			lines := strings.Split(string(out), "\n")
			for _, l := range lines {
				if strings.HasPrefix(l, "Status:") {
					s.Enabled = strings.Contains(l, "active")
				}
			}
			s.Rules = parseUFWRules(lines)
		}
	case "firewalld":
		out, err := exec.Command("firewall-cmd", "--state").CombinedOutput()
		if err == nil {
			s.Enabled = strings.TrimSpace(string(out)) == "running"
		}
	}

	return s
}

// Toggle liga/desliga o firewall.
func Toggle(enable bool) error {
	engine := Detect()
	switch engine {
	case "pf":
		state := "off"
		if enable {
			state = "on"
		}
		out, err := exec.Command("sudo", "/usr/libexec/ApplicationFirewall/socketfilterfw", "--setglobalstate", state).CombinedOutput()
		if err != nil {
			return fmt.Errorf("%v: %s", err, string(out))
		}
	case "ufw":
		action := "disable"
		if enable {
			action = "enable"
		}
		out, err := exec.Command("sudo", "ufw", "--force", action).CombinedOutput()
		if err != nil {
			return fmt.Errorf("%v: %s", err, string(out))
		}
	case "firewalld":
		if enable {
			out, err := exec.Command("sudo", "systemctl", "start", "firewalld").CombinedOutput()
			if err != nil {
				return fmt.Errorf("%v: %s", err, string(out))
			}
		} else {
			out, err := exec.Command("sudo", "systemctl", "stop", "firewalld").CombinedOutput()
			if err != nil {
				return fmt.Errorf("%v: %s", err, string(out))
			}
		}
	default:
		return fmt.Errorf("nenhum firewall detectado")
	}
	return nil
}

// AllowPort adiciona uma regra allow.
func AllowPort(port, proto string) error {
	engine := Detect()
	switch engine {
	case "ufw":
		out, err := exec.Command("sudo", "ufw", "allow", port+"/"+proto).CombinedOutput()
		if err != nil {
			return fmt.Errorf("%v: %s", err, string(out))
		}
	case "firewalld":
		out, err := exec.Command("sudo", "firewall-cmd", "--add-port="+port+"/"+proto, "--permanent").CombinedOutput()
		if err != nil {
			return fmt.Errorf("%v: %s", err, string(out))
		}
		_ = exec.Command("sudo", "firewall-cmd", "--reload").Run()
	default:
		return fmt.Errorf("operação não suportada para engine: %s", engine)
	}
	return nil
}

// DenyPort adiciona uma regra deny.
func DenyPort(port, proto string) error {
	engine := Detect()
	switch engine {
	case "ufw":
		out, err := exec.Command("sudo", "ufw", "deny", port+"/"+proto).CombinedOutput()
		if err != nil {
			return fmt.Errorf("%v: %s", err, string(out))
		}
	default:
		return fmt.Errorf("operação não suportada para engine: %s", engine)
	}
	return nil
}

func parseUFWRules(lines []string) []Rule {
	var rules []Rule
	inRules := false
	for _, l := range lines {
		if strings.HasPrefix(l, "---") {
			inRules = true
			continue
		}
		if !inRules || strings.TrimSpace(l) == "" {
			continue
		}
		parts := strings.Fields(l)
		if len(parts) >= 2 {
			rules = append(rules, Rule{
				ID:     fmt.Sprintf("%d", len(rules)+1),
				Raw:    strings.TrimSpace(l),
				Action: parts[1],
			})
		}
	}
	return rules
}
