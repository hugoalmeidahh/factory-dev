package ssh

import "strings"

// ImportableAccount representa um bloco Host do ~/.ssh/config que pode ser importado.
type ImportableAccount struct {
	HostAlias    string
	HostName     string
	User         string
	IdentityFile string
}

// ParseImportableBlocks parseia um arquivo SSH config e retorna blocos Host não-FDEV.
func ParseImportableBlocks(configPath string) ([]ImportableAccount, error) {
	parsed, err := ParseSSHConfig(configPath)
	if err != nil {
		return nil, err
	}
	var result []ImportableAccount
	for _, block := range parsed.Blocks {
		if block.IsFDev {
			continue
		}
		acc := ImportableAccount{HostAlias: block.Alias}
		for _, line := range block.Lines[1:] { // pula a linha "Host ..."
			parts := strings.Fields(line)
			if len(parts) < 2 {
				continue
			}
			key := strings.ToLower(parts[0])
			val := parts[1]
			switch key {
			case "hostname":
				acc.HostName = val
			case "user":
				acc.User = val
			case "identityfile":
				acc.IdentityFile = val
			}
		}
		result = append(result, acc)
	}
	return result, nil
}
