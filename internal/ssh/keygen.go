package ssh

import (
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"encoding/pem"
	"errors"
	"fmt"
	"os"

	"github.com/seuusuario/factorydev/internal/config"
	gossh "golang.org/x/crypto/ssh"
)

var ErrKeyExists = errors.New("chave já existe para este alias")

// KeyGenResult contém os caminhos dos arquivos de chave gerados.
type KeyGenResult struct {
	PrivateKeyPath string
	PublicKeyPath  string
}

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

// GenerateKeyFull gera uma chave com opções completas: tipo, bits e passphrase.
// Retorna os paths dos arquivos gerados.
func GenerateKeyFull(alias, comment, keyType string, bits int, passphrase []byte, paths *config.Paths) (*KeyGenResult, error) {
	if err := os.MkdirAll(paths.KeyDir(alias), 0o700); err != nil {
		return nil, err
	}
	if err := os.Chmod(paths.KeyDir(alias), 0o700); err != nil {
		return nil, err
	}
	privPath := paths.PrivateKeyForType(alias, keyType)
	pubPath := paths.PublicKeyForType(alias, keyType)
	if err := writeKeyPair(privPath, pubPath, comment, keyType, bits, passphrase); err != nil {
		return nil, err
	}
	return &KeyGenResult{PrivateKeyPath: privPath, PublicKeyPath: pubPath}, nil
}

// RegenPublicKey lê a chave privada e regenera o arquivo .pub.
func RegenPublicKey(privKeyPath, pubKeyPath string, passphrase []byte) error {
	privBytes, err := os.ReadFile(privKeyPath)
	if err != nil {
		return fmt.Errorf("ler chave privada: %w", err)
	}
	var signer gossh.Signer
	if len(passphrase) > 0 {
		signer, err = gossh.ParsePrivateKeyWithPassphrase(privBytes, passphrase)
	} else {
		signer, err = gossh.ParsePrivateKey(privBytes)
	}
	if err != nil {
		return fmt.Errorf("parsear chave privada: %w", err)
	}
	pubBytes := gossh.MarshalAuthorizedKey(signer.PublicKey())
	return os.WriteFile(pubKeyPath, pubBytes, 0o644)
}

func generateKey(alias, comment, keyType string, paths *config.Paths) error {
	if err := os.MkdirAll(paths.KeyDir(alias), 0o700); err != nil {
		return err
	}
	if err := os.Chmod(paths.KeyDir(alias), 0o700); err != nil {
		return err
	}
	privPath := paths.PrivateKeyForType(alias, keyType)
	pubPath := paths.PublicKeyForType(alias, keyType)
	return writeKeyPair(privPath, pubPath, comment, keyType, 0, nil)
}

func writeKeyPair(privPath, pubPath, comment, keyType string, bits int, passphrase []byte) error {
	switch keyType {
	case "rsa", "rsa4096":
		if bits == 0 {
			bits = 4096
		}
		return writeRSAKey(privPath, pubPath, comment, bits, passphrase)
	case "ecdsa":
		if bits == 0 {
			bits = 256
		}
		return writeECDSAKey(privPath, pubPath, comment, bits, passphrase)
	default: // "ed25519" ou ""
		return writeEd25519Key(privPath, pubPath, comment, passphrase)
	}
}

func writeEd25519Key(privPath, pubPath, comment string, passphrase []byte) error {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return err
	}
	sshPub, err := gossh.NewPublicKey(pub)
	if err != nil {
		return err
	}
	var privPEM *pem.Block
	if len(passphrase) > 0 {
		privPEM, err = gossh.MarshalPrivateKeyWithPassphrase(priv, comment, passphrase)
	} else {
		privPEM, err = gossh.MarshalPrivateKey(priv, comment)
	}
	if err != nil {
		return err
	}
	if err := os.WriteFile(privPath, pem.EncodeToMemory(privPEM), 0o600); err != nil {
		return err
	}
	return os.WriteFile(pubPath, gossh.MarshalAuthorizedKey(sshPub), 0o644)
}

func writeRSAKey(privPath, pubPath, comment string, bits int, passphrase []byte) error {
	privRSA, err := rsa.GenerateKey(rand.Reader, bits)
	if err != nil {
		return err
	}
	sshPub, err := gossh.NewPublicKey(&privRSA.PublicKey)
	if err != nil {
		return err
	}
	var privPEM *pem.Block
	if len(passphrase) > 0 {
		privPEM, err = gossh.MarshalPrivateKeyWithPassphrase(privRSA, comment, passphrase)
	} else {
		privPEM, err = gossh.MarshalPrivateKey(privRSA, comment)
	}
	if err != nil {
		return err
	}
	if err := os.WriteFile(privPath, pem.EncodeToMemory(privPEM), 0o600); err != nil {
		return err
	}
	return os.WriteFile(pubPath, gossh.MarshalAuthorizedKey(sshPub), 0o644)
}

func writeECDSAKey(privPath, pubPath, comment string, bits int, passphrase []byte) error {
	var curve elliptic.Curve
	switch bits {
	case 384:
		curve = elliptic.P384()
	case 521:
		curve = elliptic.P521()
	default: // 256
		curve = elliptic.P256()
	}
	privECDSA, err := ecdsa.GenerateKey(curve, rand.Reader)
	if err != nil {
		return err
	}
	sshPub, err := gossh.NewPublicKey(&privECDSA.PublicKey)
	if err != nil {
		return err
	}
	var privPEM *pem.Block
	if len(passphrase) > 0 {
		privPEM, err = gossh.MarshalPrivateKeyWithPassphrase(privECDSA, comment, passphrase)
	} else {
		privPEM, err = gossh.MarshalPrivateKey(privECDSA, comment)
	}
	if err != nil {
		return err
	}
	if err := os.WriteFile(privPath, pem.EncodeToMemory(privPEM), 0o600); err != nil {
		return err
	}
	return os.WriteFile(pubPath, gossh.MarshalAuthorizedKey(sshPub), 0o644)
}
