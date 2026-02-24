package storage

import (
	"crypto/rand"
	"encoding/hex"
)

func genID() string {
	buf := make([]byte, 16)
	_, _ = rand.Read(buf)
	return hex.EncodeToString(buf)
}

func migrateV1ToV2(s *State) error {
	if s.Keys == nil {
		s.Keys = []Key{}
	}
	for i := range s.Accounts {
		a := &s.Accounts[i]
		if a.KeyID != "" {
			continue
		}
		privPath := a.IdentityFile
		if privPath == "" {
			continue
		}
		kt := a.KeyType
		if kt == "" {
			kt = "ed25519"
		}
		// Normalize legacy "rsa4096" to "rsa"
		if kt == "rsa4096" {
			kt = "rsa"
		}
		bits := 0
		if kt == "rsa" {
			bits = 4096
		}
		comment := a.GitUserEmail
		if comment == "" {
			comment = a.Name
		}
		k := Key{
			ID:             genID(),
			Name:           a.Name,
			Alias:          a.HostAlias,
			Type:           kt,
			Bits:           bits,
			Comment:        comment,
			PrivateKeyPath: privPath,
			PublicKeyPath:  privPath + ".pub",
			Source:         "generated",
			CreatedAt:      a.CreatedAt,
		}
		s.Keys = append(s.Keys, k)
		a.KeyID = k.ID
	}
	return nil
}

func migrateV2ToV3(s *State) error {
	if s.Servers == nil {
		s.Servers = []Server{}
	}
	if s.Identities == nil {
		s.Identities = []GitIdentity{}
	}
	return nil
}

var migrations = map[int]func(*State) error{
	1: migrateV1ToV2,
	2: migrateV2ToV3,
}

func migrate(s *State) error {
	for s.SchemaVersion < CurrentSchema {
		fn, ok := migrations[s.SchemaVersion]
		if !ok {
			break
		}
		if err := fn(s); err != nil {
			return err
		}
		s.SchemaVersion++
	}
	return nil
}
