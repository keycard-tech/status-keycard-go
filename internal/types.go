package internal

import (
	"github.com/status-im/status-keycard-go/pkg/utils"
)

type Signature struct {
	R utils.HexString `json:"r"`
	S utils.HexString `json:"s"`
	V byte            `json:"v"`
}

type ApplicationInfo struct {
	Initialized    bool            `json:"initialized"`
	InstanceUID    utils.HexString `json:"instanceUID"`
	Version        int             `json:"version"`
	AvailableSlots int             `json:"availableSlots"`
	// KeyUID is the sha256 of the master public key on the card.
	// It's empty if the card doesn't contain any key.
	KeyUID utils.HexString `json:"keyUID"`
}

// ApplicationInfoV2 is the same as ApplicationInfo but with a string version field.
type ApplicationInfoV2 struct {
	Installed      bool            `json:"installed"`
	Initialized    bool            `json:"initialized"`
	InstanceUID    utils.HexString `json:"instanceUID"`
	versionRaw     int             `json:"-"`
	Version        string          `json:"version"`
	AvailableSlots int             `json:"availableSlots"`
	// KeyUID is the sha256 of the master public key on the card.
	// It's empty if the card doesn't contain any key.
	KeyUID utils.HexString `json:"keyUID"`
}

type KeyPair struct {
	Address    string          `json:"address"`
	PublicKey  utils.HexString `json:"publicKey"`
	PrivateKey utils.HexString `json:"privateKey,omitempty"`
	ChainCode  utils.HexString `json:"chainCode,omitempty"`
}

type Wallet struct {
	Path      string          `json:"path"`
	Address   string          `json:"address,omitempty"`
	PublicKey utils.HexString `json:"publicKey"`
}

type Metadata struct {
	Name    string   `json:"name"`
	Wallets []Wallet `json:"wallets"`
}
