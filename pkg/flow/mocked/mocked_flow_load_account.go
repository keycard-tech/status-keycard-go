package flow

import (
	"math/rand"
	"strings"

	"github.com/status-im/status-keycard-go/signal"
	"github.com/status-im/status-keycard-go/internal"
)

func (mkf *MockedKeycardFlow) handleLoadAccountFlow() {
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

	var (
		factoryReset          bool
		overwrite             bool
		enteredMnemonicLength int
		enteredMnemonic       string
		enteredNewPUK         string
		enteredPIN            string
		enteredNewPIN         string
	)

	if v, ok := mkf.params[FactoryReset]; ok {
		factoryReset = v.(bool)
	}
	if v, ok := mkf.params[Overwrite]; ok {
		overwrite = v.(bool)
	}
	if v, ok := mkf.params[MnemonicLen]; ok {
		switch t := v.(type) {
		case int:
			enteredMnemonicLength = t
		case float64:
			enteredMnemonicLength = int(t)
		default:
			enteredMnemonicLength = defMnemoLen
		}
	} else {
		enteredMnemonicLength = defMnemoLen
	}
	if v, ok := mkf.params[Mnemonic]; ok {
		enteredMnemonic = v.(string)
	}
	if v, ok := mkf.params[NewPUK]; ok {
		enteredNewPUK = v.(string)
	}
	if v, ok := mkf.params[PIN]; ok {
		enteredPIN = v.(string)
	}
	if v, ok := mkf.params[NewPIN]; ok {
		enteredNewPIN = v.(string)
	}

	if factoryReset {
		*mkf.insertedKeycard = MockedKeycard{}
	}

	if mkf.insertedKeycard.InstanceUID != "" && mkf.insertedKeycard.KeyUID != "" {
		flowStatus[internal.ErrorKey] = internal.ErrorHasKeys
		flowStatus[FreeSlots] = mkf.insertedKeycard.FreePairingSlots
		mkf.state = Paused
		signal.Send(finalType, flowStatus)
		return
	}

	if len(enteredPIN) == defPINLen && enteredPIN == enteredNewPIN && len(enteredNewPUK) == defPUKLen {
		if overwrite && enteredMnemonic == "" {

			if mkf.insertedKeycard.InstanceUID == "" {
				mkf.insertedKeycard.InstanceUID = mkf.insertedKeycardHelper.InstanceUID
				mkf.insertedKeycard.PairingInfo = mkf.insertedKeycardHelper.PairingInfo
			}

			mkf.pairings.Store(mkf.insertedKeycard.InstanceUID, mkf.insertedKeycard.PairingInfo)

			var indexes []int
			for len(indexes) < enteredMnemonicLength {
				indexes = append(indexes, rand.Intn(2048))
			}

			finalType = EnterMnemonic
			flowStatus[internal.ErrorKey] = internal.ErrorLoading
			flowStatus[MnemonicIdxs] = indexes
			flowStatus[InstanceUID] = mkf.insertedKeycard.InstanceUID
			flowStatus[FreeSlots] = mkf.insertedKeycard.FreePairingSlots
			flowStatus[PINRetries] = mkf.insertedKeycard.PinRetries
			flowStatus[PUKRetries] = mkf.insertedKeycard.PukRetries
			mkf.state = Paused
			signal.Send(finalType, flowStatus)
			return
		} else {
			realMnemonicLength := len(strings.Split(enteredMnemonic, " "))
			if enteredMnemonicLength == realMnemonicLength {
				mkf.insertedKeycard.InstanceUID = mkf.insertedKeycardHelper.InstanceUID
				mkf.insertedKeycard.PairingInfo = mkf.insertedKeycardHelper.PairingInfo
				mkf.insertedKeycard.KeyUID = mkf.insertedKeycardHelper.KeyUID
				mkf.insertedKeycard.Pin = enteredPIN
				mkf.insertedKeycard.Puk = enteredNewPUK
				mkf.insertedKeycard.PinRetries = maxPINRetries
				mkf.insertedKeycard.PukRetries = maxPUKRetries
				mkf.insertedKeycard.FreePairingSlots = maxFreeSlots - 1

				mkf.pairings.Store(mkf.insertedKeycard.InstanceUID, mkf.insertedKeycard.PairingInfo)

				finalType = FlowResult
				flowStatus[InstanceUID] = mkf.insertedKeycard.InstanceUID
				flowStatus[KeyUID] = mkf.insertedKeycard.KeyUID
				mkf.state = Idle
				signal.Send(finalType, flowStatus)
				return
			}
		}
	}

	finalType = EnterNewPIN
	flowStatus[internal.ErrorKey] = internal.ErrorRequireInit
	mkf.state = Paused
	signal.Send(finalType, flowStatus)
}
