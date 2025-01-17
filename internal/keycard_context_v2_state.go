package internal

import (
	"github.com/status-im/keycard-go/types"
)

type State string

const (
	UnknownReaderState      State = "unknown"
	NoPCSC                  State = "no-pcsc"
	InternalError           State = "internal-error"
	WaitingForReader        State = "waiting-for-reader"
	WaitingForCard          State = "waiting-for-card"
	ConnectingCard          State = "connecting-card"
	ConnectionError         State = "connection-error" // NOTE: Perhaps a good place for retry
	NotKeycard              State = "not-keycard"
	EmptyKeycard            State = "empty-keycard"
	NoAvailablePairingSlots State = "no-available-pairing-slots"
	PairingError            State = "pairing-error"
	BlockedPIN              State = "blocked-pin" // PIN remaining attempts == 0
	BlockedPUK              State = "blocked-puk" // PUK remaining attempts == 0
	Ready                   State = "ready"
	FactoryResetting        State = "factory-resetting"
)

type Status struct {
	State     State                    `json:"state"`
	AppInfo   *ApplicationInfoV2       `json:"keycardInfo"`
	AppStatus *types.ApplicationStatus `json:"keycardStatus"`
}

func NewStatus() *Status {
	status := &Status{}
	status.Reset()
	return status
}

func (s *Status) Reset() {
	s.State = UnknownReaderState
	s.AppInfo = nil
	s.AppStatus = nil
}
