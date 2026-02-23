package config

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
)

type Paths struct {
	Base    string
	Keys    string
	Logs    string
	Backups string
	State   string
	Home    string
}

var validAlias = regexp.MustCompile(`^[a-z0-9_-]+$`)

func NewPaths() (*Paths, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("obter home dir: %w", err)
	}

	base := filepath.Join(home, ".fdev")
	return &Paths{
		Base:    base,
		Keys:    filepath.Join(base, "keys"),
		Logs:    filepath.Join(base, "logs"),
		Backups: filepath.Join(base, "backups"),
		State:   filepath.Join(base, "state.json"),
		Home:    home,
	}, nil
}

func (p *Paths) KeyDir(alias string) string {
	if !validAlias.MatchString(alias) {
		panic("alias inválido: " + alias)
	}
	return filepath.Join(p.Keys, alias)
}

// keyFileName retorna o nome do arquivo para cada tipo de chave.
func keyFileName(keyType string) string {
	switch keyType {
	case "rsa", "rsa4096":
		return "id_rsa"
	case "ecdsa":
		return "id_ecdsa"
	default: // "ed25519" ou ""
		return "id_ed25519"
	}
}

// PrivateKey retorna o path Ed25519 (compatibilidade retroativa).
func (p *Paths) PrivateKey(alias string) string {
	return p.PrivateKeyForType(alias, "ed25519")
}

// PrivateKeyForType retorna o path da chave privada pelo tipo.
func (p *Paths) PrivateKeyForType(alias, keyType string) string {
	return filepath.Join(p.KeyDir(alias), keyFileName(keyType))
}

// PublicKey retorna o path Ed25519 (compatibilidade retroativa).
func (p *Paths) PublicKey(alias string) string {
	return p.PublicKeyForType(alias, "ed25519")
}

// PublicKeyForType retorna o path da chave pública pelo tipo.
func (p *Paths) PublicKeyForType(alias, keyType string) string {
	return filepath.Join(p.KeyDir(alias), keyFileName(keyType)+".pub")
}

func (p *Paths) SSHDir() string {
	return filepath.Join(p.Home, ".ssh")
}

func (p *Paths) SSHConfig() string {
	return filepath.Join(p.SSHDir(), "config")
}

func (p *Paths) ValidateAlias(alias string) bool {
	return validAlias.MatchString(alias)
}
