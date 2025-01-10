package internal

import (
	"encoding/binary"
	"encoding/hex"

	"github.com/ebfe/scard"
	keycard "github.com/status-im/keycard-go"
	"github.com/status-im/keycard-go/derivationpath"
	ktypes "github.com/status-im/keycard-go/types"
)

func IsSCardError(err error) bool {
	_, ok := err.(scard.Error)
	return ok
}

func GetRetries(err error) (int, bool) {
	if wrongPIN, ok := err.(*keycard.WrongPINError); ok {
		return wrongPIN.RemainingAttempts, ok
	} else if wrongPUK, ok := err.(*keycard.WrongPUKError); ok {
		return wrongPUK.RemainingAttempts, ok
	} else {
		return 0, false
	}
}

func Btox(bytes []byte) string {
	return hex.EncodeToString(bytes)
}

func Xtob(str string) ([]byte, error) {
	return hex.DecodeString(str)
}

func BytesToInt(s []byte) int {
	if len(s) > 4 {
		return 0
	}

	var b [4]byte
	copy(b[4-len(s):], s)
	return int(binary.BigEndian.Uint32(b[:]))
}

func ToAppInfo(r *ktypes.ApplicationInfo) ApplicationInfo {
	return ApplicationInfo{
		Initialized:    r.Initialized,
		InstanceUID:    r.InstanceUID,
		Version:        BytesToInt(r.Version),
		AvailableSlots: BytesToInt(r.AvailableSlots),
		KeyUID:         r.KeyUID,
	}
}

func ToPairInfo(r *ktypes.PairingInfo) *PairingInfo {
	return &PairingInfo{
		Key:   r.Key,
		Index: r.Index,
	}
}

func ToSignature(r *ktypes.Signature) *Signature {
	return &Signature{
		R: r.R(),
		S: r.S(),
		V: r.V(),
	}
}

func ToMetadata(r *ktypes.Metadata) *Metadata {
	paths := r.Paths()
	wallets := make([]Wallet, len(paths))

	tmp := []uint32{0x8000002c, 0x8000003c, 0x80000000, 0x00000000, 0x00000000}

	for i, p := range paths {
		tmp[4] = p
		path := derivationpath.Encode(tmp)
		wallets[i].Path = path
	}

	return &Metadata{
		Name:    r.Name(),
		Wallets: wallets,
	}
}
