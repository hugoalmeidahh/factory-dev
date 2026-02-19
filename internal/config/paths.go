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

func (p *Paths) PrivateKey(alias string) string {
	return filepath.Join(p.KeyDir(alias), "id_ed25519")
}

func (p *Paths) PublicKey(alias string) string {
	return filepath.Join(p.KeyDir(alias), "id_ed25519.pub")
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
