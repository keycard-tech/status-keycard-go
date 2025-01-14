package internal

import (
	"github.com/status-im/keycard-go/types"
)

type State string

const (
	Unknown              State = "unknown"
	NoPCSC               State = "no-pcsc"
	WaitingForReader     State = "no-reader"
	NoCard               State = "no-card"
	ConnectionError      State = "connection-error" // NOTE: Perhaps a good place for retry
	NotKeycard           State = "not-a-keycard"
	PairingError         State = "pairing-error"
	WrongPairingPassword State = "wrong-pairing-password"
	Ready                State = "ready"
)

type Status struct {
	// WARNING: Check if State is actually needed.
	// 			With further data it's redundant for most states.
	State State `json:"state"`

	Readers      []string                 `json:"readers"`
	CardInserted bool                     `json:"cardInserted"`
	AppInfo      *ApplicationInfo         `json:"cardInfo"`
	AppStatus    *types.ApplicationStatus `json:"appStatus"`
}

func NewStatus() *Status {
	status := &Status{}
	status.Reset()
	return status
}

func (s *Status) Reset() {
	s.State = Unknown
	s.Readers = []string{}
	s.CardInserted = false
	s.AppInfo = nil
	s.AppStatus = nil
}
