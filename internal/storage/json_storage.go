package storage

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

type JSONStorage struct {
	path string
}

func NewJSONStorage(path string) *JSONStorage {
	return &JSONStorage{path: path}
}

func (s *JSONStorage) LoadState() (*State, error) {
	data, err := os.ReadFile(s.path)
	if errors.Is(err, os.ErrNotExist) {
		return &State{SchemaVersion: CurrentSchema, Keys: make([]Key, 0), Accounts: make([]Account, 0), Repositories: make([]Repository, 0), Servers: make([]Server, 0), Identities: make([]GitIdentity, 0)}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("ler state: %w", err)
	}

	var state State
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, fmt.Errorf("parse state: %w", err)
	}
	if state.SchemaVersion == 0 {
		state.SchemaVersion = CurrentSchema
	}
	if state.Keys == nil {
		state.Keys = make([]Key, 0)
	}
	if state.Accounts == nil {
		state.Accounts = make([]Account, 0)
	}
	if state.Repositories == nil {
		state.Repositories = make([]Repository, 0)
	}
	if state.Servers == nil {
		state.Servers = make([]Server, 0)
	}
	if state.Identities == nil {
		state.Identities = make([]GitIdentity, 0)
	}

	if err := migrate(&state); err != nil {
		return nil, fmt.Errorf("migrar state: %w", err)
	}
	return &state, nil
}

func (s *JSONStorage) SaveState(state *State) error {
	state.UpdatedAt = time.Now()
	if state.SchemaVersion == 0 {
		state.SchemaVersion = CurrentSchema
	}

	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("serializar state: %w", err)
	}

	if err := os.MkdirAll(filepath.Dir(s.path), 0o700); err != nil {
		return fmt.Errorf("criar pasta state: %w", err)
	}

	tmp := s.path + ".tmp"
	f, err := os.OpenFile(tmp, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		return fmt.Errorf("abrir tmp state: %w", err)
	}

	if _, err = f.Write(data); err != nil {
		_ = f.Close()
		return fmt.Errorf("escrever tmp state: %w", err)
	}
	if err = f.Sync(); err != nil {
		_ = f.Close()
		return fmt.Errorf("sync tmp state: %w", err)
	}
	if err = f.Close(); err != nil {
		return fmt.Errorf("fechar tmp state: %w", err)
	}

	if err = os.Chmod(tmp, 0o600); err != nil {
		return fmt.Errorf("chmod tmp state: %w", err)
	}
	if err = os.Rename(tmp, s.path); err != nil {
		return fmt.Errorf("rename tmp state: %w", err)
	}
	if err = os.Chmod(s.path, 0o600); err != nil {
		return fmt.Errorf("chmod state final: %w", err)
	}
	return nil
}
