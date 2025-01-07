package statuskeycardgo

import (
	"encoding/binary"
	"encoding/hex"

	"github.com/ebfe/scard"
	keycard "github.com/status-im/keycard-go"
	"github.com/status-im/keycard-go/derivationpath"
	ktypes "github.com/status-im/keycard-go/types"
)

func isSCardError(err error) bool {
	_, ok := err.(scard.Error)
	return ok
}

func getRetries(err error) (int, bool) {
	if wrongPIN, ok := err.(*keycard.WrongPINError); ok {
		return wrongPIN.RemainingAttempts, ok
	} else if wrongPUK, ok := err.(*keycard.WrongPUKError); ok {
		return wrongPUK.RemainingAttempts, ok
	} else {
		return 0, false
	}
}

func btox(bytes []byte) string {
	return hex.EncodeToString(bytes)
}

func xtob(str string) ([]byte, error) {
	return hex.DecodeString(str)
}

func bytesToInt(s []byte) int {
	if len(s) > 4 {
		return 0
	}

	var b [4]byte
	copy(b[4-len(s):], s)
	return int(binary.BigEndian.Uint32(b[:]))
}

func toAppInfo(r *ktypes.ApplicationInfo) ApplicationInfo {
	return ApplicationInfo{
		Initialized:    r.Initialized,
		InstanceUID:    r.InstanceUID,
		Version:        bytesToInt(r.Version),
		AvailableSlots: bytesToInt(r.AvailableSlots),
		KeyUID:         r.KeyUID,
	}
}

func toPairInfo(r *ktypes.PairingInfo) *PairingInfo {
	return &PairingInfo{
		Key:   r.Key,
		Index: r.Index,
	}
}

func toSignature(r *ktypes.Signature) *Signature {
	return &Signature{
		R: r.R(),
		S: r.S(),
		V: r.V(),
	}
}

func toMetadata(r *ktypes.Metadata) *Metadata {
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
