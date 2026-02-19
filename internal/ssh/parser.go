package ssh

import (
	"bufio"
	"os"
	"strings"
)

type SSHConfigBlock struct {
	Alias  string
	IsFDev bool
	Lines  []string
}

type ParsedSSHConfig struct {
	Blocks      []SSHConfigBlock
	HeaderLines []string
}

func ParseSSHConfig(path string) (*ParsedSSHConfig, error) {
	f, err := os.Open(path)
	if os.IsNotExist(err) {
		return &ParsedSSHConfig{}, nil
	}
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var result ParsedSSHConfig
	var currentBlock *SSHConfigBlock
	var pendingFDevAlias string

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		trimmed := strings.TrimSpace(line)

		if strings.HasPrefix(trimmed, "# BEGIN FDEV ") {
			pendingFDevAlias = strings.TrimSpace(strings.TrimPrefix(trimmed, "# BEGIN FDEV "))
			continue
		}
		if strings.HasPrefix(trimmed, "# END FDEV ") {
			continue
		}

		if strings.HasPrefix(strings.ToLower(trimmed), "host ") {
			if currentBlock != nil {
				result.Blocks = append(result.Blocks, *currentBlock)
			}
			fields := strings.Fields(trimmed)
			alias := ""
			if len(fields) > 1 {
				alias = fields[1]
			}
			currentBlock = &SSHConfigBlock{
				Alias:  alias,
				IsFDev: pendingFDevAlias == alias,
				Lines:  []string{line},
			}
			pendingFDevAlias = ""
			continue
		}

		if currentBlock != nil {
			currentBlock.Lines = append(currentBlock.Lines, line)
		} else {
			result.HeaderLines = append(result.HeaderLines, line)
		}
	}
	if currentBlock != nil {
		result.Blocks = append(result.Blocks, *currentBlock)
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return &result, nil
}
