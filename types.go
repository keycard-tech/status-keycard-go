package statuskeycardgo

import (
	"encoding/json"
)

type hexString []byte

// MarshalJSON serializes hexString to hex
func (s hexString) MarshalJSON() ([]byte, error) {
	bytes, err := json.Marshal(btox(s))
	return bytes, err
}

// UnmarshalJSON deserializes hexString to hex
func (s *hexString) UnmarshalJSON(data []byte) error {
	var x string
	err := json.Unmarshal(data, &x)
	if err != nil {
		return err
	}
	str, err := xtob(x)
	if err != nil {
		return err
	}

	*s = hexString([]byte(str))
	return nil
}

type Signature struct {
	R hexString `json:"r"`
	S hexString `json:"s"`
	V byte      `json:"v"`
}

type ApplicationInfo struct {
	Initialized    bool      `json:"initialized"`
	InstanceUID    hexString `json:"instanceUID"`
	Version        int       `json:"version"`
	AvailableSlots int       `json:"availableSlots"`
	// KeyUID is the sha256 of of the master public key on the card.
	// It's empty if the card doesn't contain any key.
	KeyUID hexString `json:"keyUID"`
}

type PairingInfo struct {
	Key   hexString `json:"key"`
	Index int       `json:"index"`
}

type KeyPair struct {
	Address    string    `json:"address"`
	PublicKey  hexString `json:"publicKey"`
	PrivateKey hexString `json:"privateKey,omitempty"`
	ChainCode  hexString `json:"chainCode,omitempty"`
}

type Wallet struct {
	Path      string    `json:"path"`
	Address   string    `json:"address,omitempty"`
	PublicKey hexString `json:"publicKey"`
}

type Metadata struct {
	Name    string   `json:"name"`
	Wallets []Wallet `json:"wallets"`
}
