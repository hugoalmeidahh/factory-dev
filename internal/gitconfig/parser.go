package gitconfig

import (
	"bufio"
	"os"
	"strings"
)

// IncludeIfRule representa uma regra [includeIf "gitdir:/path/"] no gitconfig global.
type IncludeIfRule struct {
	Pattern     string // ex: "gitdir:/home/user/work/"
	IncludePath string // caminho do arquivo de config incluído
}

// ParseGlobalConfig lê o arquivo de gitconfig em path e retorna as chaves simples.
func ParseGlobalConfig(path string) (map[string]string, error) {
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return map[string]string{}, nil
		}
		return nil, err
	}
	defer f.Close()

	result := map[string]string{}
	var section string

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, ";") {
			continue
		}
		if strings.HasPrefix(line, "[") {
			end := strings.Index(line, "]")
			if end < 0 {
				continue
			}
			header := line[1:end]
			if strings.ContainsRune(header, ' ') {
				section = ""
				continue
			}
			section = strings.ToLower(strings.TrimSpace(header))
			continue
		}
		if section == "" {
			continue
		}
		if idx := strings.IndexByte(line, '='); idx > 0 {
			key := strings.TrimSpace(line[:idx])
			val := strings.TrimSpace(line[idx+1:])
			result[section+"."+key] = val
		}
	}
	return result, scanner.Err()
}

// ListIncludeIf lê o arquivo de gitconfig em path e retorna as regras [includeIf].
func ListIncludeIf(path string) ([]IncludeIfRule, error) {
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	defer f.Close()

	var rules []IncludeIfRule
	var currentPattern string

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, ";") {
			continue
		}
		if strings.HasPrefix(line, "[") {
			end := strings.Index(line, "]")
			if end < 0 {
				currentPattern = ""
				continue
			}
			header := line[1:end]
			lower := strings.ToLower(header)
			if strings.HasPrefix(lower, "includeif ") {
				q1 := strings.Index(header, `"`)
				q2 := strings.LastIndex(header, `"`)
				if q1 >= 0 && q2 > q1 {
					currentPattern = header[q1+1 : q2]
				} else {
					currentPattern = ""
				}
			} else {
				currentPattern = ""
			}
			continue
		}
		if currentPattern == "" {
			continue
		}
		if idx := strings.IndexByte(line, '='); idx > 0 {
			key := strings.ToLower(strings.TrimSpace(line[:idx]))
			val := strings.TrimSpace(line[idx+1:])
			if key == "path" {
				rules = append(rules, IncludeIfRule{
					Pattern:     currentPattern,
					IncludePath: val,
				})
				currentPattern = ""
			}
		}
	}
	return rules, scanner.Err()
}
