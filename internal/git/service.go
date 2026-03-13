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

// PullForce executa git fetch --all + git reset --hard @{u},
// descartando mudanças locais e sincronizando com o upstream.
func (s *Service) PullForce(ctx context.Context, localPath, identityFile string) (string, error) {
	var env []string
	if identityFile != "" {
		env = append(os.Environ(),
			"GIT_SSH_COMMAND=ssh -i "+identityFile+" -o IdentitiesOnly=yes -o StrictHostKeyChecking=accept-new",
		)
	}

	fetchCmd := exec.CommandContext(ctx, "git", "-C", localPath, "fetch", "--all")
	if len(env) > 0 {
		fetchCmd.Env = env
	}
	fetchOut, fetchErr := fetchCmd.CombinedOutput()
	if fetchErr != nil {
		return string(fetchOut), fmt.Errorf("fetch: %w", fetchErr)
	}

	resetCmd := exec.CommandContext(ctx, "git", "-C", localPath, "reset", "--hard", "@{u}")
	resetOut, resetErr := resetCmd.CombinedOutput()
	combined := string(fetchOut) + string(resetOut)
	if resetErr != nil {
		return combined, fmt.Errorf("reset: %w", resetErr)
	}
	return combined, nil
}

// BranchInfo representa um branch local com status de sincronização.
type BranchInfo struct {
	Name        string
	Current     bool
	Ahead       int
	Behind      int
	HasUpstream bool
}

// ListBranches lista os branches locais com status ahead/behind do upstream.
func (s *Service) ListBranches(ctx context.Context, localPath string) ([]BranchInfo, error) {
	cmd := exec.CommandContext(ctx, "git", "-C", localPath, "branch", "-vv",
		"--format=%(refname:short)|%(HEAD)|%(upstream:short)|%(upstream:track)")
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("git branch: %w", err)
	}
	var branches []BranchInfo
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "|", 4)
		if len(parts) < 2 {
			continue
		}
		name := strings.TrimSpace(parts[0])
		current := strings.TrimSpace(parts[1]) == "*"
		upstream := ""
		track := ""
		if len(parts) >= 3 {
			upstream = strings.TrimSpace(parts[2])
		}
		if len(parts) >= 4 {
			track = strings.TrimSpace(parts[3])
		}
		bi := BranchInfo{Name: name, Current: current, HasUpstream: upstream != ""}
		if track != "" {
			if a := parseTrackNum(track, "ahead"); a > 0 {
				bi.Ahead = a
			}
			if b := parseTrackNum(track, "behind"); b > 0 {
				bi.Behind = b
			}
		}
		branches = append(branches, bi)
	}
	return branches, nil
}

// parseTrackNum extrai o número de commits ahead ou behind do formato [ahead N, behind M].
func parseTrackNum(track, dir string) int {
	// track looks like "[ahead 2]", "[behind 1]", "[ahead 2, behind 3]"
	track = strings.Trim(track, "[]")
	for _, part := range strings.Split(track, ",") {
		part = strings.TrimSpace(part)
		if strings.HasPrefix(part, dir+" ") {
			rest := strings.TrimPrefix(part, dir+" ")
			n := 0
			for _, ch := range rest {
				if ch >= '0' && ch <= '9' {
					n = n*10 + int(ch-'0')
				}
			}
			return n
		}
	}
	return 0
}

// CheckoutBranch faz checkout de um branch existente.
func (s *Service) CheckoutBranch(ctx context.Context, localPath, branch string) error {
	cmd := exec.CommandContext(ctx, "git", "-C", localPath, "checkout", branch)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s: %w", strings.TrimSpace(string(out)), err)
	}
	return nil
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

// DefaultScanExcludes são os diretórios ignorados por padrão no scan.
var DefaultScanExcludes = []string{
	"node_modules", ".cache", ".npm", "vendor", "dist", "build",
	".next", ".nuxt", "out", "target", "__pycache__", ".tox",
	".venv", "venv", ".gradle", ".idea", ".vscode",
}

// ScanForRepos escaneia root procurando diretórios git até maxDepth de profundidade.
// excludeDirs são nomes de diretórios a ignorar.
func (s *Service) ScanForRepos(root string, maxDepth int, excludeDirs []string) ([]string, error) {
	root, err := expandHome(root)
	if err != nil {
		return nil, err
	}
	excluded := make(map[string]bool, len(excludeDirs))
	for _, d := range excludeDirs {
		excluded[d] = true
	}
	var found []string
	err = filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return filepath.SkipDir
		}
		if !d.IsDir() {
			return nil
		}
		if excluded[d.Name()] {
			return filepath.SkipDir
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
