package mocked

import (
	"github.com/status-im/status-keycard-go/internal"
	"github.com/status-im/status-keycard-go/pkg/pairing"
)

type MockedReaderState int

const (
	NoReader MockedReaderState = iota
	NoKeycard
	KeycardInserted
)

type MockedKeycardState int

const (
	NotStatusKeycard MockedKeycardState = iota
	EmptyKeycard
	MaxPairingSlotsReached
	MaxPINRetriesReached
	MaxPUKRetriesReached
	KeycardWithMnemonicOnly
	KeycardWithMnemonicAndMedatada
)

type MockedKeycard struct {
	PairingInfo      *pairing.Info               `json:"pairing-info"`
	NotStatusKeycard bool                        `json:"not-status-keycard"`
	InstanceUID      string                      `json:"instance-uid"`
	KeyUID           string                      `json:"key-uid"`
	FreePairingSlots int                         `json:"free-pairing-slots"`
	PinRetries       int                         `json:"pin-retries"`
	PukRetries       int                         `json:"puk-retries"`
	Pin              string                      `json:"pin"`
	Puk              string                      `json:"puk"`
	Metadata         internal.Metadata           `json:"card-metadata"`
	MasterKeyAddress string                      `json:"master-key-address"` // used to predefine master key address in specific flows (like ExportPublic)
	ExportedKey      map[string]internal.KeyPair `json:"exported-key"`       // [path]KeyPair - used to predefine adderss/private/public keys in specific flows (like ExportPublic)
}

var mockedKeycard = MockedKeycard{
	InstanceUID:      "00000000000000000000001234567890",
	KeyUID:           "0000000000000000000000000000000000000000000000000000001234567890",
	FreePairingSlots: internal.MaxFreeSlots - 1,
	PinRetries:       internal.MaxPINRetries,
	PukRetries:       internal.MaxPUKRetries,
	Pin:              "111111",
	Puk:              "111111111111",
	PairingInfo: &pairing.Info{
		Key:   []byte("0000000000000000000000000000000000000000000000000000001111111111"),
		Index: 0,
	},
	Metadata: internal.Metadata{
		Name: "Card-1 Name",
		Wallets: []internal.Wallet{
			{
				Path:      "m/44'/60'/0'/0/0",
				Address:   "0x0000000000000000000000000000000000000001",
				PublicKey: []byte("0000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000001"),
			},
			{
				Path:      "m/44'/60'/0'/0/1",
				Address:   "0x0000000000000000000000000000000000000002",
				PublicKey: []byte("0000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000002"),
			},
		},
	},
}

var mockedKeycardHelper = MockedKeycard{
	InstanceUID:      "00000000000000000000001234567890",
	KeyUID:           "0000000000000000000000000000000000000000000000000000001234567890",
	FreePairingSlots: internal.MaxFreeSlots - 1,
	PinRetries:       internal.MaxPINRetries,
	PukRetries:       internal.MaxPUKRetries,
	Pin:              "111111",
	Puk:              "111111111111",
	PairingInfo: &pairing.Info{
		Key:   []byte("0000000000000000000000000000000000000000000000000000001111111111"),
		Index: 0,
	},
	Metadata: internal.Metadata{
		Name: "Card-1 Name",
		Wallets: []internal.Wallet{
			{
				Path:      "m/44'/60'/0'/0/0",
				Address:   "0x0000000000000000000000000000000000000001",
				PublicKey: []byte("0000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000001"),
			},
			{
				Path:      "m/44'/60'/0'/0/1",
				Address:   "0x0000000000000000000000000000000000000002",
				PublicKey: []byte("0000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000002"),
			},
		},
	},
	MasterKeyAddress: "0x0000000000000000000000000000000000000100",
	ExportedKey: map[string]internal.KeyPair{
		"m/44'/60'/0'/0/0": {
			Address:    "0x0000000000000000000000000000000000000001",
			PublicKey:  []byte("0000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000001"),
			PrivateKey: []byte("0000000000000000000000000000000000000000000000000000000000000001"),
		},
		"m/44'/60'/0'/0/1": {
			Address:    "0x0000000000000000000000000000000000000002",
			PublicKey:  []byte("0000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000002"),
			PrivateKey: []byte("0000000000000000000000000000000000000000000000000000000000000002"),
		},
	},
}
