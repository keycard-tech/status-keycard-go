package internal

import (
	"encoding/binary"
	"fmt"

	"github.com/ebfe/scard"
	"github.com/status-im/keycard-go"
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

func BytesToInt(s []byte) int {
	if len(s) > 4 {
		return 0
	}

	var b [4]byte
	copy(b[4-len(s):], s)
	return int(binary.BigEndian.Uint32(b[:]))
}

func ContainsString(str string, s []string) bool {
	for _, v := range s {
		if v == str {
			return true
		}
	}

	return false
}

func ToAppInfo(r *ktypes.ApplicationInfo) ApplicationInfo {
	if r == nil {
		return ApplicationInfo{}
	}
	return ApplicationInfo{
		Initialized:    r.Initialized,
		InstanceUID:    r.InstanceUID,
		Version:        BytesToInt(r.Version),
		AvailableSlots: BytesToInt(r.AvailableSlots),
		KeyUID:         r.KeyUID,
	}
}

func ParseVersion(input []byte) string {
	if len(input) == 0 {
		return ""
	}
	if len(input) != 2 {
		return "unexpected version format"
	}

	major := input[0]
	minor := input[1]
	return fmt.Sprintf("%d.%d", major, minor)
}

func ToAppInfoV2(r *ktypes.ApplicationInfo) *ApplicationInfoV2 {
	if r == nil {
		return nil
	}
	return &ApplicationInfoV2{
		Installed:      r.Installed,
		Initialized:    r.Initialized,
		InstanceUID:    r.InstanceUID,
		versionRaw:     BytesToInt(r.Version),
		Version:        ParseVersion(r.Version),
		AvailableSlots: BytesToInt(r.AvailableSlots),
		KeyUID:         r.KeyUID,
	}
}

func ToAppStatus(r *ktypes.ApplicationStatus) *ApplicationStatus {
	if r == nil {
		return nil
	}
	return &ApplicationStatus{
		RemainingAttemptsPIN: r.PinRetryCount,
		RemainingAttemptsPUK: r.PUKRetryCount,
		KeyInitialized:       r.KeyInitialized,
		Path:                 r.Path,
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
