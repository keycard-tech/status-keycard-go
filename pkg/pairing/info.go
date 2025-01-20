package pairing

import (
	ktypes "github.com/status-im/keycard-go/types"

	"github.com/status-im/status-keycard-go/pkg/utils"
)

type Info struct {
	Key   utils.HexString `json:"key"`
	Index int             `json:"index"`
}

func ToPairInfo(r *ktypes.PairingInfo) *Info {
	return &Info{
		Key:   r.Key,
		Index: r.Index,
	}
}
