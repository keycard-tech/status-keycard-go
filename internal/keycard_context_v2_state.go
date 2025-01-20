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
	ConnectionError         State = "connection-error"
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
	Metadata  *Metadata                `json:"metadata"`
}

func NewStatus() *Status {
	status := &Status{}
	status.Reset(UnknownReaderState)
	return status
}

func (s *Status) Reset(newState State) {
	s.State = newState
	s.AppInfo = nil
	s.AppStatus = nil
	s.Metadata = nil
}

func (s *Status) KeycardSupportsExtendedKeys() bool {
	return s.AppInfo != nil && s.AppInfo.versionRaw >= 0x0310
}
