package statuskeycardgo

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

type pairingStore struct {
	path   string
	values map[string]*PairingInfo
}

func newPairingStore(storage string) (*pairingStore, error) {
	if storage == "" {
		l("storage path was empty")
		return nil, errors.New("storage path cannot be empty")
	}

	p := &pairingStore{
		path:   filepath.Clean(storage),
		values: make(map[string]*PairingInfo),
	}

	if err := os.MkdirAll(filepath.Dir(p.path), 0755); err != nil {
		l("failed to create directory: %w", err)
		return nil, fmt.Errorf("failed to create directory: %w", err)
	}

	b, err := os.ReadFile(p.path)
	if err != nil {
		if !os.IsNotExist(err) {
			l("failed to read file: %w", err)
			return nil, fmt.Errorf("failed to read file: %w", err)
		}
		// File doesn't exist, which is fine for a new store
	} else {
		if err := json.Unmarshal(b, &p.values); err != nil {
			l("failed to parse JSON: %w", err)
			return nil, fmt.Errorf("failed to parse JSON: %w", err)
		}
	}

	return p, nil
}

func (p *pairingStore) save() error {
	b, err := json.Marshal(p.values)

	if err != nil {
		return err
	}

	err = os.WriteFile(p.path, b, 0640)

	if err != nil {
		return err
	}

	return nil
}

func (p *pairingStore) store(instanceUID string, pairing *PairingInfo) error {
	p.values[instanceUID] = pairing
	return p.save()
}

func (p *pairingStore) get(instanceUID string) *PairingInfo {
	return p.values[instanceUID]
}

func (p *pairingStore) delete(instanceUID string) {
	delete(p.values, instanceUID)
}
