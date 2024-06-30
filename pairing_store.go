package statuskeycardgo

import (
	"encoding/json"
	"os"
	"path/filepath"
)

type pairingStore struct {
	path   string
	values map[string]*PairingInfo
}

func newPairingStore(storage string) (*pairingStore, error) {
	p := &pairingStore{
		path:   storage,
		values: make(map[string]*PairingInfo),
	}

	b, err := os.ReadFile(p.path)
	if err != nil {
		if os.IsNotExist(err) {
			parent := filepath.Dir(p.path)
			err = os.MkdirAll(parent, 0750)
			if err != nil {
				return nil, err
			}
		} else {
			return nil, err
		}
	} else {
		err = json.Unmarshal(b, &p.values)
		if err != nil {
			p.values = make(map[string]*PairingInfo)
			l("error at newPairingStore is %+v", err)
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
