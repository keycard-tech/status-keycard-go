package statuskeycardgo

import (
	"encoding/json"
	"github.com/status-im/status-keycard-go/internal"
)

type HexString []byte

// MarshalJSON serializes HexString to hex
func (s HexString) MarshalJSON() ([]byte, error) {
	bytes, err := json.Marshal(internal.Btox(s))
	return bytes, err
}

// UnmarshalJSON deserializes HexString to hex
func (s *HexString) UnmarshalJSON(data []byte) error {
	var x string
	err := json.Unmarshal(data, &x)
	if err != nil {
		return err
	}
	str, err := internal.Xtob(x)
	if err != nil {
		return err
	}

	*s = HexString([]byte(str))
	return nil
}

type Signature struct {
	R HexString `json:"r"`
	S HexString `json:"s"`
	V byte      `json:"v"`
}

type ApplicationInfo struct {
	Initialized    bool      `json:"initialized"`
	InstanceUID    HexString `json:"instanceUID"`
	Version        int       `json:"version"`
	AvailableSlots int       `json:"availableSlots"`
	// KeyUID is the sha256 of the master public key on the card.
	// It's empty if the card doesn't contain any key.
	KeyUID HexString `json:"keyUID"`
}

type PairingInfo struct {
	Key   HexString `json:"key"`
	Index int       `json:"index"`
}

type KeyPair struct {
	Address    string    `json:"address"`
	PublicKey  HexString `json:"publicKey"`
	PrivateKey HexString `json:"privateKey,omitempty"`
}

type Wallet struct {
	Path      string    `json:"path"`
	Address   string    `json:"address,omitempty"`
	PublicKey HexString `json:"publicKey"`
}

type Metadata struct {
	Name    string   `json:"name"`
	Wallets []Wallet `json:"wallets"`
}
