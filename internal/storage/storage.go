package storage

import "time"

const CurrentSchema = 3

type Storage interface {
	LoadState() (*State, error)
	SaveState(*State) error
}

type Key struct {
	ID             string    `json:"id"`
	Name           string    `json:"name"`
	Alias          string    `json:"alias"`          // dir ~/.fdev/keys/<alias>/
	Type           string    `json:"type"`           // "ed25519", "rsa", "ecdsa"
	Bits           int       `json:"bits,omitempty"` // RSA 2048/3072/4096; ECDSA 256/384/521
	Comment        string    `json:"comment"`
	PrivateKeyPath string    `json:"privateKeyPath"`
	PublicKeyPath  string    `json:"publicKeyPath"`
	Protected      bool      `json:"protected"`      // tem passphrase
	Source         string    `json:"source"`         // "generated" | "imported"
	OriginalPath   string    `json:"originalPath,omitempty"`
	CreatedAt      time.Time `json:"createdAt"`
}

type EnvFile struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	ProjectPath string            `json:"projectPath,omitempty"`
	Variables   map[string]string `json:"variables"`
	CreatedAt   time.Time         `json:"createdAt"`
	UpdatedAt   time.Time         `json:"updatedAt"`
}

type ShellAlias struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Command   string    `json:"command"`
	CreatedAt time.Time `json:"createdAt"`
}

type APICollection struct {
	ID        string            `json:"id"`
	Name      string            `json:"name"`
	AuthType  string            `json:"authType"`           // "none","bearer","basic","apikey"
	AuthData  map[string]string `json:"authData,omitempty"` // token, username, password, headerName, headerValue
	EnvVars   map[string]string `json:"envVars,omitempty"`
	CreatedAt time.Time         `json:"createdAt"`
}

type APIEndpoint struct {
	ID           string            `json:"id"`
	CollectionID string            `json:"collectionId"`
	Name         string            `json:"name"`
	Method       string            `json:"method"` // GET, POST, PUT, DELETE, PATCH
	URL          string            `json:"url"`
	Headers      map[string]string `json:"headers,omitempty"`
	Body         string            `json:"body,omitempty"`
	CreatedAt    time.Time         `json:"createdAt"`
	UpdatedAt    time.Time         `json:"updatedAt"`
}

type APIRequestHistory struct {
	ID           string    `json:"id"`
	EndpointID   string    `json:"endpointId,omitempty"`
	Method       string    `json:"method"`
	URL          string    `json:"url"`
	StatusCode   int       `json:"statusCode"`
	ResponseTime int64     `json:"responseTimeMs"`
	RequestedAt  time.Time `json:"requestedAt"`
}

type DBConnection struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Driver    string    `json:"driver"` // "sqlite","postgres","mysql"
	Host      string    `json:"host,omitempty"`
	Port      int       `json:"port,omitempty"`
	User      string    `json:"user,omitempty"`
	Password  string    `json:"password,omitempty"`
	Database  string    `json:"database"`
	SSLMode   string    `json:"sslMode,omitempty"`
	CreatedAt time.Time `json:"createdAt"`
}

type MCPServer struct {
	ID        string            `json:"id"`
	Name      string            `json:"name"`
	Command   string            `json:"command"`
	Args      []string          `json:"args,omitempty"`
	Env       map[string]string `json:"env,omitempty"`
	Enabled   bool              `json:"enabled"`
	CreatedAt time.Time         `json:"createdAt"`
}

type CustomSkill struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Prompt      string    `json:"prompt"`
	Tags        []string  `json:"tags,omitempty"`
	CreatedAt   time.Time `json:"createdAt"`
}

type State struct {
	SchemaVersion  int                 `json:"schemaVersion"`
	Keys           []Key               `json:"keys"`
	Accounts       []Account           `json:"accounts"`
	Repositories   []Repository        `json:"repositories"`
	Servers        []Server            `json:"servers"`
	Identities     []GitIdentity       `json:"identities"`
	EnvFiles       []EnvFile           `json:"envFiles,omitempty"`
	Aliases        []ShellAlias        `json:"aliases,omitempty"`
	APICollections []APICollection     `json:"apiCollections,omitempty"`
	APIEndpoints   []APIEndpoint       `json:"apiEndpoints,omitempty"`
	APIHistory     []APIRequestHistory `json:"apiHistory,omitempty"`
	DBConnections  []DBConnection      `json:"dbConnections,omitempty"`
	MCPServers     []MCPServer         `json:"mcpServers,omitempty"`
	CustomSkills   []CustomSkill       `json:"customSkills,omitempty"`
	UpdatedAt      time.Time           `json:"updatedAt"`
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
	KeyID        string    `json:"keyId,omitempty"`
	GitUserName  string    `json:"gitUserName"`
	GitUserEmail string    `json:"gitUserEmail"`
	// Legacy fields — kept as omitempty for backward compat / migration
	IdentityFile string    `json:"identityFile,omitempty"`
	KeyType      string    `json:"keyType,omitempty"`
	IsSimpleKey  bool      `json:"isSimpleKey,omitempty"`
	CreatedAt    time.Time `json:"createdAt"`
	UpdatedAt    time.Time `json:"updatedAt"`
}

type GitIdentity struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Email     string    `json:"email"`
	KeyID     string    `json:"keyId,omitempty"` // referência a Key para commit signing
	CreatedAt time.Time `json:"createdAt"`
}

type Server struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Host        string    `json:"host"`
	Port        int       `json:"port"` // default 22
	User        string    `json:"user"`
	KeyID       string    `json:"keyId,omitempty"`
	Description string    `json:"description,omitempty"`
	Tags        []string  `json:"tags,omitempty"`
	CreatedAt   time.Time `json:"createdAt"`
}

// EffectiveKeyType retorna o tipo da chave legado, defaultando para "ed25519".
func (a Account) EffectiveKeyType() string {
	if a.KeyType == "" {
		return "ed25519"
	}
	return a.KeyType
}
