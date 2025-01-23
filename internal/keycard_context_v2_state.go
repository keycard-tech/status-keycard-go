package internal

import (
	"errors"
)

type State string

const (
	// UnknownReaderState is the default state when the monitoring was not started.
	UnknownReaderState State = "unknown"

	// NoPCSC - PCSC library was not found. Can only happen during Start.
	NoPCSC State = "no-pcsc"

	// InternalError - an internal error occurred.
	// Should never happen, check logs for more details. Depending on circumstances, can stop the monitoring when occurred.
	InternalError State = "internal-error"

	// WaitingForReader - no reader was found.
	WaitingForReader State = "waiting-for-reader"

	// WaitingForCard - no card was found inserted into any of connected readers.
	WaitingForCard State = "waiting-for-card"

	// ConnectingCard - a card was found inserted into a reader and the connection is being established.
	// This state is usually very short, as the connection is established quickly.
	ConnectingCard State = "connecting-card"

	// ConnectionError - an error occurred while connecting or communicating with the card.
	// In all cases, the monitoring will continue to stay in the watch mode and expect the user to reinsert the card.
	ConnectionError State = "connection-error"

	// NotKeycard - the card inserted is not a keycard (does not have Keycard applet installed)
	NotKeycard State = "not-keycard"

	// EmptyKeycard - the keycard is empty, i.e. has not been initialized (PIN/PUK are not set).
	// Use Initialize command to initialize the keycard.
	EmptyKeycard State = "empty-keycard"

	// NoAvailablePairingSlots - there are no available pairing slots on the keycard.
	// Use Unpair command to unpair an existing slot (this command must be executed from the paired devices),
	// or use FactoryReset command to reset the keycard to factory settings.
	NoAvailablePairingSlots State = "no-available-pairing-slots"

	// PairingError - an error occurred during the pairing process.
	// This can be due to a wrong pairing password.
	PairingError State = "pairing-error"

	// BlockedPIN - the PIN is blocked (remaining attempts == 0).
	// Use UnblockPIN command to unblock the PIN.
	BlockedPIN State = "blocked-pin"

	// BlockedPUK - the PUK is blocked (remaining attempts == 0).
	// The keycard is completely blocked. Use FactoryReset command to reset the keycard to factory settings
	// and recover the keycard with recovery phrase.
	BlockedPUK State = "blocked-puk"

	// Ready - the keycard is ready for use.
	// The keycard is initialized, paired and secure channel is established.
	// The PIN has not been verified, so only unauthenticated commands can be executed.
	Ready State = "ready"

	// Authorized - the keycard is authorized (PIN verified).
	// The keycard is in Ready state and the PIN has been verified, allowing authenticated commands to be executed.
	Authorized State = "authorized"

	// FactoryResetting - the keycard is undergoing a factory reset.
	// The keycard is being reset to factory settings. This process can take a few seconds.
	FactoryResetting State = "factory-resetting"
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
