package statuskeycardgo

import (
	"encoding/json"
	"os"
	"path/filepath"
	"github.com/status-im/status-keycard-go/internal"
)

type pairingStore struct {
	path   string
	values map[string]*internal.PairingInfo
}

func newPairingStore(storage string) (*pairingStore, error) {
	p := &pairingStore{path: storage}
	b, err := os.ReadFile(p.path)

	if err != nil {
		if os.IsNotExist(err) {
			parent := filepath.Dir(p.path)
			err = os.MkdirAll(parent, 0750)

			if err != nil {
				return nil, err
			}

			p.values = map[string]*internal.PairingInfo{}
		} else {
			return nil, err
		}
	} else {
		err = json.Unmarshal(b, &p.values)

		if err != nil {
			return nil, err
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

func (p *pairingStore) store(instanceUID string, pairing *internal.PairingInfo) error {
	p.values[instanceUID] = pairing
	return p.save()
}

func (p *pairingStore) get(instanceUID string) *internal.PairingInfo {
	return p.values[instanceUID]
}

func (p *pairingStore) delete(instanceUID string) {
	delete(p.values, instanceUID)
}
