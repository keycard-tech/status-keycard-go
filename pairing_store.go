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

	l("attempting to read filepath is %+v", p.path)
	b, err := os.ReadFile(p.path)
	l("error at os.ReadFile with path logged above is %+v", err)
	if err != nil {
		if os.IsNotExist(err) {
			parent := filepath.Dir(p.path)
			err = os.MkdirAll(parent, 0755)
			if err != nil {
				l("error at os.MkdirAll(parent, 0755) is %+v", err)
				return nil, err
			}
		} else {
			l("error at !os.IsNotExist(err) is %+v", err)
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
	l("no error at newPairingStore and value of pairingStore is %+v", p)
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
