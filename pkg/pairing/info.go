package pairing

import (
	"github.com/status-im/status-keycard-go/pkg/utils"
	ktypes "github.com/status-im/keycard-go/types"
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
