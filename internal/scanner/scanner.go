package scanner

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/seuusuario/factorydev/internal/config"
	"github.com/seuusuario/factorydev/internal/storage"
)

// UnmanagedKey representa uma chave em ~/.fdev/keys/ que não tem conta associada.
type UnmanagedKey struct {
	Alias string
	Path  string
}

// UnmanagedRepo representa um repo git encontrado no filesystem não gerenciado pelo FactoryDev.
type UnmanagedRepo struct {
	Name string
	Path string
}

// Result agrega os resultados do scan.
type Result struct {
	UnmanagedKeys  []UnmanagedKey
	UnmanagedRepos []UnmanagedRepo
}

func (r *Result) Total() int {
	return len(r.UnmanagedKeys) + len(r.UnmanagedRepos)
}

func (r *Result) HasUnmanaged() bool {
	return r.Total() > 0
}

// Scan escaneia chaves não gerenciadas em ~/.fdev/keys/ e repos git não registrados
// nos diretórios padrão de workspace do usuário.
func Scan(paths *config.Paths, state *storage.State) *Result {
	return &Result{
		UnmanagedKeys:  scanUnmanagedKeys(paths, state.Accounts),
		UnmanagedRepos: scanUnmanagedRepos(paths.Home, state.Repositories),
	}
}

func scanUnmanagedKeys(paths *config.Paths, accounts []storage.Account) []UnmanagedKey {
	knownAliases := make(map[string]bool, len(accounts))
	for _, a := range accounts {
		if a.HostAlias != "" {
			knownAliases[a.HostAlias] = true
		}
	}

	entries, err := os.ReadDir(paths.Keys)
	if err != nil {
		return nil
	}

	var unmanaged []UnmanagedKey
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		alias := entry.Name()
		if !knownAliases[alias] {
			unmanaged = append(unmanaged, UnmanagedKey{
				Alias: alias,
				Path:  filepath.Join(paths.Keys, alias),
			})
		}
	}
	return unmanaged
}

// workspaceDirs são os diretórios padrão de projetos que serão escaneados.
var workspaceDirs = []string{"workspace", "projects", "code", "dev", "src"}

func scanUnmanagedRepos(homeDir string, repos []storage.Repository) []UnmanagedRepo {
	knownPaths := make(map[string]bool, len(repos))
	for _, r := range repos {
		knownPaths[filepath.Clean(r.LocalPath)] = true
	}

	var unmanaged []UnmanagedRepo
	seen := make(map[string]bool)

	for _, dir := range workspaceDirs {
		wsDir := filepath.Join(homeDir, dir)
		if _, err := os.Stat(wsDir); err != nil {
			continue
		}
		_ = filepath.WalkDir(wsDir, func(path string, d os.DirEntry, err error) error {
			if err != nil {
				return filepath.SkipDir
			}
			if !d.IsDir() {
				return nil
			}
			// Pula diretórios ocultos exceto .git
			if strings.HasPrefix(d.Name(), ".") && d.Name() != ".git" {
				return filepath.SkipDir
			}
			if d.Name() == ".git" {
				repoPath := filepath.Clean(filepath.Dir(path))
				if !knownPaths[repoPath] && !seen[repoPath] {
					seen[repoPath] = true
					unmanaged = append(unmanaged, UnmanagedRepo{
						Name: filepath.Base(repoPath),
						Path: repoPath,
					})
				}
				return filepath.SkipDir
			}
			return nil
		})
	}
	return unmanaged
}
