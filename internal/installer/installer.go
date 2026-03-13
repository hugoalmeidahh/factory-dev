package installer

import (
	"os/exec"
	"runtime"
)

// Tool descreve uma ferramenta instalável.
type Tool struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	BrewName    string `json:"brewName,omitempty"`
	AptName     string `json:"aptName,omitempty"`
	DnfName     string `json:"dnfName,omitempty"`
	CheckBin    string `json:"checkBin"` // binário para LookPath
}

// Manifest é a lista fixa de ferramentas suportadas.
var Manifest = []Tool{
	{Name: "git", Description: "Sistema de controle de versão", BrewName: "git", AptName: "git", DnfName: "git", CheckBin: "git"},
	{Name: "docker", Description: "Plataforma de containers", BrewName: "docker", AptName: "docker.io", DnfName: "docker", CheckBin: "docker"},
	{Name: "node", Description: "Runtime JavaScript (Node.js)", BrewName: "node", AptName: "nodejs", DnfName: "nodejs", CheckBin: "node"},
	{Name: "python3", Description: "Linguagem Python 3", BrewName: "python@3", AptName: "python3", DnfName: "python3", CheckBin: "python3"},
	{Name: "go", Description: "Linguagem Go", BrewName: "go", AptName: "golang", DnfName: "golang", CheckBin: "go"},
	{Name: "curl", Description: "Transferência de dados via URL", BrewName: "curl", AptName: "curl", DnfName: "curl", CheckBin: "curl"},
	{Name: "wget", Description: "Download de arquivos", BrewName: "wget", AptName: "wget", DnfName: "wget", CheckBin: "wget"},
	{Name: "jq", Description: "Processador JSON de linha de comando", BrewName: "jq", AptName: "jq", DnfName: "jq", CheckBin: "jq"},
	{Name: "htop", Description: "Monitor de processos interativo", BrewName: "htop", AptName: "htop", DnfName: "htop", CheckBin: "htop"},
	{Name: "tmux", Description: "Multiplexador de terminal", BrewName: "tmux", AptName: "tmux", DnfName: "tmux", CheckBin: "tmux"},
	{Name: "vim", Description: "Editor de texto", BrewName: "vim", AptName: "vim", DnfName: "vim-enhanced", CheckBin: "vim"},
	{Name: "make", Description: "Ferramenta de build", BrewName: "make", AptName: "make", DnfName: "make", CheckBin: "make"},
	{Name: "tree", Description: "Visualização de diretórios", BrewName: "tree", AptName: "tree", DnfName: "tree", CheckBin: "tree"},
	{Name: "httpie", Description: "Cliente HTTP amigável", BrewName: "httpie", AptName: "httpie", DnfName: "httpie", CheckBin: "http"},
	{Name: "gh", Description: "GitHub CLI", BrewName: "gh", AptName: "gh", DnfName: "gh", CheckBin: "gh"},
}

// ToolStatus representa o estado de uma ferramenta.
type ToolStatus struct {
	Tool
	Installed bool   `json:"installed"`
	Path      string `json:"path,omitempty"`
}

// IsInstalled verifica se o binário existe no PATH.
func IsInstalled(t Tool) (bool, string) {
	path, err := exec.LookPath(t.CheckBin)
	if err != nil {
		return false, ""
	}
	return true, path
}

// PackageManager detecta o gerenciador de pacotes do sistema.
type PackageManager string

const (
	PMBrew PackageManager = "brew"
	PMApt  PackageManager = "apt"
	PMDnf  PackageManager = "dnf"
	PMNone PackageManager = ""
)

// DetectPM retorna o package manager disponível.
func DetectPM() PackageManager {
	if runtime.GOOS == "darwin" {
		if _, err := exec.LookPath("brew"); err == nil {
			return PMBrew
		}
		return PMNone
	}
	if _, err := exec.LookPath("apt-get"); err == nil {
		return PMApt
	}
	if _, err := exec.LookPath("dnf"); err == nil {
		return PMDnf
	}
	return PMNone
}

// InstallCmd retorna o comando completo para instalar a ferramenta.
func InstallCmd(t Tool, pm PackageManager) (string, []string) {
	switch pm {
	case PMBrew:
		return "brew", []string{"install", t.BrewName}
	case PMApt:
		return "sudo", []string{"apt-get", "install", "-y", t.AptName}
	case PMDnf:
		return "sudo", []string{"dnf", "install", "-y", t.DnfName}
	default:
		return "", nil
	}
}

// UninstallCmd retorna o comando completo para desinstalar a ferramenta.
func UninstallCmd(t Tool, pm PackageManager) (string, []string) {
	switch pm {
	case PMBrew:
		return "brew", []string{"uninstall", t.BrewName}
	case PMApt:
		return "sudo", []string{"apt-get", "remove", "-y", t.AptName}
	case PMDnf:
		return "sudo", []string{"dnf", "remove", "-y", t.DnfName}
	default:
		return "", nil
	}
}
