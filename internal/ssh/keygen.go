package ssh

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/rsa"
	"encoding/pem"
	"errors"
	"os"

	"github.com/seuusuario/factorydev/internal/config"
	"golang.org/x/crypto/ssh"
)

var ErrKeyExists = errors.New("chave já existe para este alias")

// GenerateKey gera uma nova chave do tipo especificado. Retorna ErrKeyExists se já existe.
func GenerateKey(alias, comment, keyType string, paths *config.Paths) error {
	privPath := paths.PrivateKeyForType(alias, keyType)
	if _, err := os.Stat(privPath); err == nil {
		return ErrKeyExists
	}
	return generateKey(alias, comment, keyType, paths)
}

// ForceGenerateKey remove e recria a chave, mesmo que já exista.
func ForceGenerateKey(alias, comment, keyType string, paths *config.Paths) error {
	_ = os.Remove(paths.PrivateKeyForType(alias, keyType))
	_ = os.Remove(paths.PublicKeyForType(alias, keyType))
	return generateKey(alias, comment, keyType, paths)
}

func generateKey(alias, comment, keyType string, paths *config.Paths) error {
	if err := os.MkdirAll(paths.KeyDir(alias), 0o700); err != nil {
		return err
	}
	if err := os.Chmod(paths.KeyDir(alias), 0o700); err != nil {
		return err
	}
	switch keyType {
	case "rsa4096":
		return generateRSAKey(alias, comment, paths)
	default: // "ed25519" ou ""
		return generateEd25519Key(alias, comment, paths)
	}
}

func generateEd25519Key(alias, comment string, paths *config.Paths) error {
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
	privPath := paths.PrivateKeyForType(alias, "ed25519")
	if err := os.WriteFile(privPath, pem.EncodeToMemory(privPEM), 0o600); err != nil {
		return err
	}
	pubBytes := ssh.MarshalAuthorizedKey(sshPub)
	return os.WriteFile(paths.PublicKeyForType(alias, "ed25519"), pubBytes, 0o644)
}

func generateRSAKey(alias, comment string, paths *config.Paths) error {
	privRSA, err := rsa.GenerateKey(rand.Reader, 4096)
	if err != nil {
		return err
	}
	sshPub, err := ssh.NewPublicKey(&privRSA.PublicKey)
	if err != nil {
		return err
	}
	privPEM, err := ssh.MarshalPrivateKey(privRSA, comment)
	if err != nil {
		return err
	}
	privPath := paths.PrivateKeyForType(alias, "rsa4096")
	if err := os.WriteFile(privPath, pem.EncodeToMemory(privPEM), 0o600); err != nil {
		return err
	}
	pubBytes := ssh.MarshalAuthorizedKey(sshPub)
	return os.WriteFile(paths.PublicKeyForType(alias, "rsa4096"), pubBytes, 0o644)
}
