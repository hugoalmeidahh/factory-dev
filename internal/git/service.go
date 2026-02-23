package git

import (
	"context"
	"fmt"
	"io"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

type Service struct{}

func NewService() *Service {
	return &Service{}
}

func (s *Service) BuildSSHURL(rawURL, hostAlias string) (string, error) {
	u := strings.TrimSpace(rawURL)
	if u == "" {
		return "", fmt.Errorf("URL do repositório é obrigatória")
	}

	if strings.HasPrefix(u, "git@") {
		parts := strings.SplitN(strings.TrimPrefix(u, "git@"), ":", 2)
		if len(parts) != 2 || parts[1] == "" {
			return "", fmt.Errorf("URL SSH inválida")
		}
		return "git@" + hostAlias + ":" + parts[1], nil
	}

	if strings.HasPrefix(u, "ssh://") {
		parsed, err := url.Parse(u)
		if err != nil {
			return "", fmt.Errorf("URL inválida: %w", err)
		}
		path := strings.TrimPrefix(parsed.Path, "/")
		if path == "" {
			return "", fmt.Errorf("URL sem caminho de repositório")
		}
		return "git@" + hostAlias + ":" + path, nil
	}

	if strings.HasPrefix(u, "https://") || strings.HasPrefix(u, "http://") {
		parsed, err := url.Parse(u)
		if err != nil {
			return "", fmt.Errorf("URL inválida: %w", err)
		}
		path := strings.TrimPrefix(parsed.Path, "/")
		if path == "" {
			return "", fmt.Errorf("URL sem caminho de repositório")
		}
		return "git@" + hostAlias + ":" + path, nil
	}

	return "", fmt.Errorf("formato de URL não suportado")
}

func (s *Service) CloneRepo(ctx context.Context, sshURL, destDir, identityFile string) (string, error) {
	target, err := expandHome(destDir)
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(target) == "" {
		return "", fmt.Errorf("diretório de destino é obrigatório")
	}
	target, err = resolveCloneTarget(target, sshURL)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return "", fmt.Errorf("criar diretório base: %w", err)
	}

	cmd := exec.CommandContext(ctx, "git", "clone", sshURL, target)
	cmd.Env = append(os.Environ(),
		"GIT_SSH_COMMAND=ssh -i "+identityFile+" -o IdentitiesOnly=yes -o StrictHostKeyChecking=accept-new",
	)
	out, err := cmd.CombinedOutput()
	return string(out), err
}

func resolveCloneTarget(destDir, sshURL string) (string, error) {
	st, err := os.Stat(destDir)
	if err == nil {
		if !st.IsDir() {
			return "", fmt.Errorf("destino existe e não é diretório: %s", destDir)
		}
		empty, err := isDirEmpty(destDir)
		if err != nil {
			return "", fmt.Errorf("verificar diretório de destino: %w", err)
		}
		if empty {
			return destDir, nil
		}
		repoName := repoDirNameFromURL(sshURL)
		if repoName == "" {
			return "", fmt.Errorf("não foi possível inferir nome do repositório da URL")
		}
		nested := filepath.Join(destDir, repoName)
		if _, err := os.Stat(nested); err == nil {
			return "", fmt.Errorf("diretório de destino já contém '%s'", repoName)
		} else if !os.IsNotExist(err) {
			return "", fmt.Errorf("verificar subdiretório de destino: %w", err)
		}
		return nested, nil
	}

	if os.IsNotExist(err) {
		return destDir, nil
	}
	return "", fmt.Errorf("acessar diretório de destino: %w", err)
}

func isDirEmpty(path string) (bool, error) {
	f, err := os.Open(path)
	if err != nil {
		return false, err
	}
	defer f.Close()
	_, err = f.Readdirnames(1)
	if err == io.EOF {
		return true, nil
	}
	return false, err
}

func repoDirNameFromURL(raw string) string {
	s := strings.TrimSpace(raw)
	if s == "" {
		return ""
	}
	if strings.HasPrefix(s, "git@") {
		parts := strings.SplitN(strings.TrimPrefix(s, "git@"), ":", 2)
		if len(parts) == 2 {
			return cleanRepoName(filepath.Base(parts[1]))
		}
		return ""
	}
	if strings.HasPrefix(s, "ssh://") || strings.HasPrefix(s, "https://") || strings.HasPrefix(s, "http://") {
		u, err := url.Parse(s)
		if err != nil {
			return ""
		}
		return cleanRepoName(filepath.Base(strings.Trim(u.Path, "/")))
	}
	return cleanRepoName(filepath.Base(s))
}

func cleanRepoName(name string) string {
	name = strings.TrimSpace(name)
	name = strings.TrimSuffix(name, ".git")
	return name
}

// CommitEntry representa um commit do histórico git.
type CommitEntry struct {
	Hash    string
	Subject string
	Author  string
	Date    string
}

// Pull executa git pull no repositório local.
func (s *Service) Pull(ctx context.Context, localPath, identityFile string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", "-C", localPath, "pull")
	if identityFile != "" {
		cmd.Env = append(os.Environ(),
			"GIT_SSH_COMMAND=ssh -i "+identityFile+" -o IdentitiesOnly=yes -o StrictHostKeyChecking=accept-new",
		)
	}
	out, err := cmd.CombinedOutput()
	return string(out), err
}

// NewBranch cria e faz checkout de um novo branch.
func (s *Service) NewBranch(ctx context.Context, localPath, branch string) error {
	cmd := exec.CommandContext(ctx, "git", "-C", localPath, "checkout", "-b", branch)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s: %w", strings.TrimSpace(string(out)), err)
	}
	return nil
}

// GetLog retorna os últimos n commits a partir do offset.
func (s *Service) GetLog(ctx context.Context, localPath string, n, offset int) ([]CommitEntry, error) {
	args := []string{"-C", localPath, "log",
		"--format=%H|%s|%an|%ad", "--date=short",
		fmt.Sprintf("-n %d", n),
		fmt.Sprintf("--skip=%d", offset),
	}
	cmd := exec.CommandContext(ctx, "git", args...)
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	var entries []CommitEntry
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "|", 4)
		if len(parts) < 4 {
			continue
		}
		entries = append(entries, CommitEntry{
			Hash:    parts[0][:min(len(parts[0]), 8)],
			Subject: parts[1],
			Author:  parts[2],
			Date:    parts[3],
		})
	}
	return entries, nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// GetRepoGitConfig retorna as configurações git locais do repositório.
func (s *Service) GetRepoGitConfig(localPath string) (map[string]string, error) {
	cmd := exec.Command("git", "-C", localPath, "config", "--list", "--local")
	out, err := cmd.Output()
	if err != nil {
		// Sem config local não é erro
		return map[string]string{}, nil
	}
	result := map[string]string{}
	for _, line := range strings.Split(string(out), "\n") {
		kv := strings.SplitN(line, "=", 2)
		if len(kv) == 2 {
			result[strings.TrimSpace(kv[0])] = strings.TrimSpace(kv[1])
		}
	}
	return result, nil
}

// SetRepoGitConfig define um valor de config git local no repositório.
func (s *Service) SetRepoGitConfig(localPath, key, value string) error {
	cmd := exec.Command("git", "-C", localPath, "config", key, value)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git config %s: %s", key, strings.TrimSpace(string(out)))
	}
	return nil
}

// ScanForRepos escaneia root procurando diretórios git até maxDepth de profundidade.
func (s *Service) ScanForRepos(root string, maxDepth int) ([]string, error) {
	root, err := expandHome(root)
	if err != nil {
		return nil, err
	}
	var found []string
	err = filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return filepath.SkipDir
		}
		if !d.IsDir() {
			return nil
		}
		rel, _ := filepath.Rel(root, path)
		depth := len(strings.Split(rel, string(os.PathSeparator)))
		if rel == "." {
			depth = 0
		}
		if depth > maxDepth {
			return filepath.SkipDir
		}
		gitDir := filepath.Join(path, ".git")
		if _, err := os.Stat(gitDir); err == nil {
			found = append(found, path)
			return filepath.SkipDir // não entra em subrepos
		}
		return nil
	})
	return found, err
}

var tildePathRe = regexp.MustCompile(`^~($|/)`)

func expandHome(p string) (string, error) {
	p = strings.TrimSpace(p)
	if p == "" {
		return "", nil
	}
	if !tildePathRe.MatchString(p) {
		return p, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolver home dir: %w", err)
	}
	if p == "~" {
		return home, nil
	}
	return filepath.Join(home, strings.TrimPrefix(p, "~/")), nil
}
