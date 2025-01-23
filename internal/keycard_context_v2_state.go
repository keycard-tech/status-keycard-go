package internal

import (
	"errors"
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
	Authorized              State = "authorized" // PIN verified
	FactoryResetting        State = "factory-resetting"
)

type Status struct {
	State     State              `json:"state"`
	AppInfo   *ApplicationInfoV2 `json:"keycardInfo"`
	AppStatus *ApplicationStatus `json:"keycardStatus"`
	Metadata  *Metadata          `json:"metadata"`
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

var (
	simulatedNoPCSC                 = errors.New("simulated-no-pcsc")
	simulatedListReadersError       = errors.New("simulated-list-readers-error")
	simulatedGetStatusChangeError   = errors.New("simulated-get-status-change-error")
	simulatedCardConnectError       = errors.New("simulated-card-connect-error")
	simulatedGetCardStatusError     = errors.New("simulated-get-card-status-error")
	simulatedSelectAppletError      = errors.New("simulated-select-applet-error")
	simulatedNotAKeycard            = errors.New("simulated-not-a-keycard")
	simulatedOpenSecureChannelError = errors.New("simulated-open-secure-channel-error")
)

func GetSimulatedError(message string) error {
	errs := map[string]error{
		simulatedNoPCSC.Error():                 simulatedNoPCSC,
		simulatedListReadersError.Error():       simulatedListReadersError,
		simulatedGetStatusChangeError.Error():   simulatedGetStatusChangeError,
		simulatedCardConnectError.Error():       simulatedCardConnectError,
		simulatedGetCardStatusError.Error():     simulatedGetCardStatusError,
		simulatedSelectAppletError.Error():      simulatedSelectAppletError,
		simulatedNotAKeycard.Error():            simulatedNotAKeycard,
		simulatedOpenSecureChannelError.Error(): simulatedOpenSecureChannelError,
	}
	return errs[message]
}
