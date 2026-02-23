package ssh

import (
	"encoding/pem"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/seuusuario/factorydev/internal/config"
	gossh "golang.org/x/crypto/ssh"
)

// ImportCandidate representa um par de chaves encontrado durante o scan de diretório.
type ImportCandidate struct {
	Name        string // nome do arquivo privado (ex: "id_ed25519")
	PrivatePath string
	PublicPath  string
	Type        string // "ed25519", "rsa", "ecdsa", "unknown"
	Bits        int
	Protected   bool // protegida por passphrase
}

// ScanDir escaneia um diretório buscando pares de chaves SSH (id_* / id_*.pub).
func ScanDir(dir string) ([]ImportCandidate, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var result []ImportCandidate
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if strings.HasSuffix(name, ".pub") {
			continue
		}
		if !strings.HasPrefix(name, "id_") {
			continue
		}
		privPath := filepath.Join(dir, name)
		pubPath := privPath + ".pub"
		if _, err := os.Stat(pubPath); os.IsNotExist(err) {
			continue
		}
		privBytes, err := os.ReadFile(privPath)
		if err != nil {
			continue
		}
		kt, bits, protected := DetectKeyType(privBytes)
		result = append(result, ImportCandidate{
			Name:        name,
			PrivatePath: privPath,
			PublicPath:  pubPath,
			Type:        kt,
			Bits:        bits,
			Protected:   protected,
		})
	}
	return result, nil
}

// DetectKeyType detecta o tipo, tamanho em bits e se a chave é protegida por passphrase.
func DetectKeyType(privBytes []byte) (keyType string, bits int, protected bool) {
	block, _ := pem.Decode(privBytes)
	if block == nil {
		return "unknown", 0, false
	}

	// Tenta parsear chave não protegida
	signer, err := gossh.ParsePrivateKey(privBytes)
	if err != nil {
		// Verifica se é erro de passphrase
		errStr := err.Error()
		if strings.Contains(errStr, "passphrase") ||
			strings.Contains(errStr, "encrypted") ||
			strings.Contains(errStr, "cannot decode") {
			protected = true
		}
		// Inferir tipo pelo header PEM
		switch block.Type {
		case "RSA PRIVATE KEY":
			return "rsa", 0, true
		case "EC PRIVATE KEY":
			return "ecdsa", 0, true
		default:
			return "ed25519", 0, true
		}
	}

	// Chave não protegida — detectar pelo tipo da chave pública
	switch signer.PublicKey().Type() {
	case "ecdsa-sha2-nistp256":
		return "ecdsa", 256, false
	case "ecdsa-sha2-nistp384":
		return "ecdsa", 384, false
	case "ecdsa-sha2-nistp521":
		return "ecdsa", 521, false
	case "ssh-ed25519":
		return "ed25519", 0, false
	case "ssh-rsa":
		return "rsa", 0, false
	}
	return "unknown", 0, false
}

// CopyKeyPair copia um par de chaves para o diretório de chaves do factorydev.
// Retorna os paths de destino (priv, pub).
func CopyKeyPair(src ImportCandidate, destAlias string, paths *config.Paths) (string, string, error) {
	if err := os.MkdirAll(paths.KeyDir(destAlias), 0o700); err != nil {
		return "", "", err
	}

	var privDestName string
	switch src.Type {
	case "rsa":
		privDestName = "id_rsa"
	case "ecdsa":
		privDestName = "id_ecdsa"
	default:
		privDestName = "id_ed25519"
	}

	privDest := filepath.Join(paths.KeyDir(destAlias), privDestName)
	pubDest := privDest + ".pub"

	if err := copyKeyFile(src.PrivatePath, privDest, 0o600); err != nil {
		return "", "", err
	}
	if err := copyKeyFile(src.PublicPath, pubDest, 0o644); err != nil {
		return "", "", err
	}
	return privDest, pubDest, nil
}

func copyKeyFile(src, dst string, mode os.FileMode) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, mode)
	if err != nil {
		return err
	}
	defer out.Close()
	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return out.Sync()
}
