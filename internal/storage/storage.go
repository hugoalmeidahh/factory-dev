package storage

import "time"

const CurrentSchema = 1

type Storage interface {
	LoadState() (*State, error)
	SaveState(*State) error
}

type State struct {
	SchemaVersion int       `json:"schemaVersion"`
	Accounts      []Account `json:"accounts"`
	UpdatedAt     time.Time `json:"updatedAt"`
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
	CreatedAt    time.Time `json:"createdAt"`
	UpdatedAt    time.Time `json:"updatedAt"`
}
