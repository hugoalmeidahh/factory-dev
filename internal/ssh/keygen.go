package ssh

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/pem"
	"errors"
	"os"

	"github.com/seuusuario/factorydev/internal/config"
	"golang.org/x/crypto/ssh"
)

var ErrKeyExists = errors.New("chave já existe para este alias")

func GenerateKey(alias, comment string, paths *config.Paths) error {
	privPath := paths.PrivateKey(alias)
	if _, err := os.Stat(privPath); err == nil {
		return ErrKeyExists
	}
	if err := os.MkdirAll(paths.KeyDir(alias), 0o700); err != nil {
		return err
	}
	if err := os.Chmod(paths.KeyDir(alias), 0o700); err != nil {
		return err
	}

	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return err
	}
	sshPub, err := ssh.NewPublicKey(pub)
	if err != nil {
		return err
	}
	privPEM, err := ssh.MarshalPrivateKey(priv, comment)
	if err != nil {
		return err
	}

	if err := os.WriteFile(privPath, pem.EncodeToMemory(privPEM), 0o600); err != nil {
		return err
	}
	pubBytes := ssh.MarshalAuthorizedKey(sshPub)
	return os.WriteFile(paths.PublicKey(alias), pubBytes, 0o644)
}
