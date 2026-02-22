package storage

import (
	"regexp"
	"strings"
)

type ValidationError struct {
	Field   string
	Message string
}

var (
	validAliasRe   = regexp.MustCompile(`^[a-z0-9_-]+$`)
	validEmailRe   = regexp.MustCompile(`^[^@\s]+@[^@\s]+\.[^@\s]+$`)
	validHostRe    = regexp.MustCompile(`^[^\s:/]+(\:[0-9]+)?$`)
	validProviders = map[string]bool{
		"github":    true,
		"gitlab":    true,
		"bitbucket": true,
		"other":     true,
	}
)

// Validate valida um Account no modo completo (com config SSH).
func Validate(a Account, existing []Account) []ValidationError {
	var errs []ValidationError

	if strings.TrimSpace(a.Name) == "" {
		errs = append(errs, ValidationError{Field: "name", Message: "obrigatório"})
	}
	if !validProviders[a.Provider] {
		errs = append(errs, ValidationError{Field: "provider", Message: "valor inválido"})
	}

	host := strings.TrimSpace(a.HostName)
	if host == "" {
		errs = append(errs, ValidationError{Field: "hostName", Message: "obrigatório"})
	} else {
		if strings.HasPrefix(strings.ToLower(host), "http://") || strings.HasPrefix(strings.ToLower(host), "https://") {
			errs = append(errs, ValidationError{Field: "hostName", Message: "não use protocolo"})
		} else if !validHostRe.MatchString(host) {
			errs = append(errs, ValidationError{Field: "hostName", Message: "formato inválido"})
		}
	}

	if !validAliasRe.MatchString(a.HostAlias) {
		errs = append(errs, ValidationError{Field: "hostAlias", Message: "apenas letras minúsculas, números, - e _"})
	}
	for _, e := range existing {
		if e.HostAlias == a.HostAlias && e.ID != a.ID {
			errs = append(errs, ValidationError{Field: "hostAlias", Message: "já existe"})
			break
		}
	}

	if strings.TrimSpace(a.GitUserName) == "" {
		errs = append(errs, ValidationError{Field: "gitUserName", Message: "obrigatório"})
	}
	if !validEmailRe.MatchString(strings.TrimSpace(a.GitUserEmail)) {
		errs = append(errs, ValidationError{Field: "gitUserEmail", Message: "email inválido"})
	}

	return errs
}

// ValidateSimple valida apenas nome e alias (modo simples, sem config SSH completa).
func ValidateSimple(a Account, existing []Account) []ValidationError {
	var errs []ValidationError

	if strings.TrimSpace(a.Name) == "" {
		errs = append(errs, ValidationError{Field: "name", Message: "obrigatório"})
	}
	if !validAliasRe.MatchString(a.HostAlias) {
		errs = append(errs, ValidationError{Field: "hostAlias", Message: "apenas letras minúsculas, números, - e _"})
	}
	for _, e := range existing {
		if e.HostAlias == a.HostAlias && e.ID != a.ID {
			errs = append(errs, ValidationError{Field: "hostAlias", Message: "já existe"})
			break
		}
	}

	return errs
}
