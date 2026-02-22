package storage

import "time"

const CurrentSchema = 1

type Storage interface {
	LoadState() (*State, error)
	SaveState(*State) error
}

type State struct {
	SchemaVersion int          `json:"schemaVersion"`
	Accounts      []Account    `json:"accounts"`
	Repositories  []Repository `json:"repositories"`
	UpdatedAt     time.Time    `json:"updatedAt"`
}

type Repository struct {
	ID        string    `json:"id"`
	AccountID string    `json:"accountId"`
	Name      string    `json:"name"`
	URL       string    `json:"url"`
	LocalPath string    `json:"localPath"`
	ClonedAt  time.Time `json:"clonedAt"`
}

type Account struct {
	ID           string    `json:"id"`
	Name         string    `json:"name"`
	Provider     string    `json:"provider"`
	HostName     string    `json:"hostName"`
	HostAlias    string    `json:"hostAlias"`
	IdentityFile string    `json:"identityFile"`
	GitUserName  string    `json:"gitUserName"`
	GitUserEmail string    `json:"gitUserEmail"`
	KeyType      string    `json:"keyType"`     // "ed25519" (padrão) ou "rsa4096"
	IsSimpleKey  bool      `json:"isSimpleKey"` // true = criado sem config SSH completa
	CreatedAt    time.Time `json:"createdAt"`
	UpdatedAt    time.Time `json:"updatedAt"`
}

// EffectiveKeyType retorna o tipo da chave, defaultando para "ed25519".
func (a Account) EffectiveKeyType() string {
	if a.KeyType == "" {
		return "ed25519"
	}
	return a.KeyType
}
