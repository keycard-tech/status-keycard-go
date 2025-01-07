package mocked

import (
	"math/rand"
	"strings"

	"github.com/status-im/status-keycard-go/signal"
	"github.com/status-im/status-keycard-go/internal"
	"github.com/status-im/status-keycard-go/pkg/flow"
)

func (mkf *MockedKeycardFlow) handleLoadAccountFlow() {
	flowStatus := flow.FlowStatus{}

	if mkf.insertedKeycard.NotStatusKeycard {
		flowStatus[internal.ErrorKey] = internal.ErrorNotAKeycard
		flowStatus[flow.InstanceUID] = ""
		flowStatus[flow.KeyUID] = ""
		flowStatus[flow.FreeSlots] = 0
		mkf.state = flow.Paused
		signal.Send(flow.SwapCard, flowStatus)
		return
	}

	finalType := flow.SwapCard
	flowStatus = flow.FlowStatus{
		flow.InstanceUID: mkf.insertedKeycard.InstanceUID,
		flow.KeyUID:      mkf.insertedKeycard.KeyUID,
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

	if v, ok := mkf.params[flow.FactoryReset]; ok {
		factoryReset = v.(bool)
	}
	if v, ok := mkf.params[flow.Overwrite]; ok {
		overwrite = v.(bool)
	}
	if v, ok := mkf.params[flow.MnemonicLen]; ok {
		switch t := v.(type) {
		case int:
			enteredMnemonicLength = t
		case float64:
			enteredMnemonicLength = int(t)
		default:
			enteredMnemonicLength = flow.DefMnemoLen
		}
	} else {
		enteredMnemonicLength = flow.DefMnemoLen
	}
	if v, ok := mkf.params[flow.Mnemonic]; ok {
		enteredMnemonic = v.(string)
	}
	if v, ok := mkf.params[flow.NewPUK]; ok {
		enteredNewPUK = v.(string)
	}
	if v, ok := mkf.params[flow.PIN]; ok {
		enteredPIN = v.(string)
	}
	if v, ok := mkf.params[flow.NewPIN]; ok {
		enteredNewPIN = v.(string)
	}

	if factoryReset {
		*mkf.insertedKeycard = MockedKeycard{}
	}

	if mkf.insertedKeycard.InstanceUID != "" && mkf.insertedKeycard.KeyUID != "" {
		flowStatus[internal.ErrorKey] = internal.ErrorHasKeys
		flowStatus[flow.FreeSlots] = mkf.insertedKeycard.FreePairingSlots
		mkf.state = flow.Paused
		signal.Send(finalType, flowStatus)
		return
	}

	if len(enteredPIN) == flow.DefPINLen && enteredPIN == enteredNewPIN && len(enteredNewPUK) == flow.DefPUKLen {
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

			finalType = flow.EnterMnemonic
			flowStatus[internal.ErrorKey] = internal.ErrorLoading
			flowStatus[flow.MnemonicIdxs] = indexes
			flowStatus[flow.InstanceUID] = mkf.insertedKeycard.InstanceUID
			flowStatus[flow.FreeSlots] = mkf.insertedKeycard.FreePairingSlots
			flowStatus[flow.PINRetries] = mkf.insertedKeycard.PinRetries
			flowStatus[flow.PUKRetries] = mkf.insertedKeycard.PukRetries
			mkf.state = flow.Paused
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
				mkf.insertedKeycard.PinRetries = flow.MaxPINRetries
				mkf.insertedKeycard.PukRetries = flow.MaxPUKRetries
				mkf.insertedKeycard.FreePairingSlots = flow.MaxFreeSlots - 1

				mkf.pairings.Store(mkf.insertedKeycard.InstanceUID, mkf.insertedKeycard.PairingInfo)

				finalType = flow.FlowResult
				flowStatus[flow.InstanceUID] = mkf.insertedKeycard.InstanceUID
				flowStatus[flow.KeyUID] = mkf.insertedKeycard.KeyUID
				mkf.state = flow.Idle
				signal.Send(finalType, flowStatus)
				return
			}
		}
	}

	finalType = flow.EnterNewPIN
	flowStatus[internal.ErrorKey] = internal.ErrorRequireInit
	mkf.state = flow.Paused
	signal.Send(finalType, flowStatus)
}
