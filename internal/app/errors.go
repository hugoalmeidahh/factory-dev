package app

import (
	"errors"
	"strings"

	fdevssh "github.com/seuusuario/factorydev/internal/ssh"
)

type AppError struct {
	Code    string
	Message string
	Err     error
}

func (e *AppError) Error() string {
	return e.Message
}

func WrapError(err error) *AppError {
	if err == nil {
		return nil
	}
	return &AppError{Message: FriendlyMessage(err), Err: err}
}

func FriendlyMessage(err error) string {
	if err == nil {
		return ""
	}
	s := strings.ToLower(err.Error())
	switch {
	case strings.Contains(s, "permission denied"):
		return "Permissão negada: verifique as permissões de ~/.ssh/config"
	case strings.Contains(s, "no such file"):
		return "Arquivo não encontrado"
	case errors.Is(err, fdevssh.ErrKeyExists):
		return "Chave já existe para este alias. Deseja sobrescrever?"
	case strings.Contains(s, "no reachable address"):
		return "Não foi possível conectar ao host"
	default:
		return "Ocorreu um erro inesperado. Verifique os logs."
	}
}
