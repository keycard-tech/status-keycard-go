package statuskeycardgo

import (
	"github.com/status-im/status-keycard-go/signal"
	"github.com/status-im/status-keycard-go/internal"
)

func (mkf *MockedKeycardFlow) handleRecoverAccountFlow() {
	flowStatus := FlowStatus{}

	if mkf.insertedKeycard.NotStatusKeycard {
		flowStatus[internal.ErrorKey] = internal.ErrorNotAKeycard
		flowStatus[InstanceUID] = ""
		flowStatus[KeyUID] = ""
		flowStatus[FreeSlots] = 0
		mkf.state = Paused
		signal.Send(SwapCard, flowStatus)
		return
	}

	finalType := SwapCard
	flowStatus = FlowStatus{
		InstanceUID: mkf.insertedKeycard.InstanceUID,
		KeyUID:      mkf.insertedKeycard.KeyUID,
	}

	if mkf.insertedKeycard.InstanceUID == "" || mkf.insertedKeycard.KeyUID == "" {
		finalType = SwapCard
		flowStatus[internal.ErrorKey] = internal.ErrorNoKeys
		flowStatus[FreeSlots] = 0
		mkf.state = Paused
		signal.Send(finalType, flowStatus)
		return
	}

	var (
		enteredPIN    string
		enteredNewPIN string
		enteredPUK    string
	)

	if v, ok := mkf.params[PIN]; ok {
		enteredPIN = v.(string)
	}
	if v, ok := mkf.params[NewPIN]; ok {
		enteredNewPIN = v.(string)
	}
	if v, ok := mkf.params[PUK]; ok {
		enteredPUK = v.(string)
	}

	finalType = EnterPIN
	if mkf.insertedKeycard.PukRetries == 0 {
		flowStatus[internal.ErrorKey] = PUKRetries
		finalType = SwapCard
	} else {
		if mkf.insertedKeycard.PinRetries == 0 {
			if len(enteredPUK) == defPUKLen {
				if len(enteredPIN) == defPINLen && enteredPIN == enteredNewPIN {
					if enteredPUK != mkf.insertedKeycard.Puk {
						mkf.insertedKeycard.PukRetries--
						if mkf.insertedKeycard.PukRetries == 0 {
							flowStatus[internal.ErrorKey] = PUKRetries
							finalType = SwapCard
						} else {
							flowStatus[internal.ErrorKey] = PUK
							finalType = EnterPUK
						}
					}
				} else {
					flowStatus[internal.ErrorKey] = internal.ErrorUnblocking
					finalType = EnterNewPIN
				}
			} else {
				flowStatus[internal.ErrorKey] = ""
				finalType = EnterPUK
			}
		} else {
			if len(enteredNewPIN) == 0 && len(enteredPIN) == defPINLen && enteredPIN != mkf.insertedKeycard.Pin {
				mkf.insertedKeycard.PinRetries--
				flowStatus[internal.ErrorKey] = PIN
				finalType = EnterPIN
				if mkf.insertedKeycard.PinRetries == 0 {
					flowStatus[internal.ErrorKey] = ""
					finalType = EnterPUK
				}
			}
		}
	}

	if mkf.insertedKeycard.PinRetries > 0 && len(enteredPIN) == defPINLen && enteredPIN == mkf.insertedKeycard.Pin ||
		mkf.insertedKeycard.PinRetries == 0 && mkf.insertedKeycard.PukRetries > 0 && len(enteredPUK) == defPUKLen &&
			enteredPUK == mkf.insertedKeycard.Puk && len(enteredPIN) == defPINLen && enteredPIN == enteredNewPIN {

		mkf.insertedKeycard.PinRetries = maxPINRetries
		mkf.insertedKeycard.PukRetries = maxPUKRetries
		mkf.insertedKeycard.Pin = enteredPIN
		flowStatus[internal.ErrorKey] = ""
		flowStatus[MasterKey] = mkf.insertedKeycardHelper.ExportedKey[masterPath]
		flowStatus[WalleRootKey] = mkf.insertedKeycardHelper.ExportedKey[walletRoothPath]
		flowStatus[WalletKey] = mkf.insertedKeycardHelper.ExportedKey[walletPath]
		flowStatus[EIP1581Key] = mkf.insertedKeycardHelper.ExportedKey[eip1581Path]
		flowStatus[WhisperKey] = mkf.insertedKeycardHelper.ExportedKey[whisperPath]
		flowStatus[EncKey] = mkf.insertedKeycardHelper.ExportedKey[encryptionPath]
		mkf.state = Idle
		signal.Send(FlowResult, flowStatus)
		return
	}

	flowStatus[FreeSlots] = mkf.insertedKeycard.FreePairingSlots
	flowStatus[PINRetries] = mkf.insertedKeycard.PinRetries
	flowStatus[PUKRetries] = mkf.insertedKeycard.PukRetries
	mkf.state = Paused
	signal.Send(finalType, flowStatus)
}
