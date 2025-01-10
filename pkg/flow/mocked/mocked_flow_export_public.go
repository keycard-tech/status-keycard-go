package mocked

import (
	"math/rand"
	"strconv"
	"strings"

	"github.com/status-im/status-keycard-go/signal"
	"github.com/status-im/status-keycard-go/internal"
	"github.com/status-im/status-keycard-go/pkg/flow"
)

func (mkf *MockedKeycardFlow) handleExportPublicFlow() {
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

	flowStatus = flow.FlowStatus{
		flow.InstanceUID: mkf.insertedKeycard.InstanceUID,
		flow.KeyUID:      mkf.insertedKeycard.KeyUID,
	}

	if mkf.insertedKeycard.InstanceUID == "" || mkf.insertedKeycard.KeyUID == "" {
		flowStatus[internal.ErrorKey] = internal.ErrorNoKeys
		flowStatus[flow.FreeSlots] = 0
		mkf.state = flow.Paused
		signal.Send(flow.SwapCard, flowStatus)
		return
	}

	var (
		enteredPIN    string
		enteredNewPIN string
		enteredPUK    string
		exportMaster  bool
		exportPrivate bool
	)

	if v, ok := mkf.params[flow.PIN]; ok {
		enteredPIN = v.(string)
	}
	if v, ok := mkf.params[flow.NewPIN]; ok {
		enteredNewPIN = v.(string)
	}
	if v, ok := mkf.params[flow.PUK]; ok {
		enteredPUK = v.(string)
	}
	if v, ok := mkf.params[flow.ExportMaster]; ok {
		exportMaster = v.(bool)
	}
	if v, ok := mkf.params[flow.ExportPriv]; ok {
		exportPrivate = v.(bool)
	}

	finalType := flow.EnterPIN
	if mkf.insertedKeycard.PukRetries == 0 {
		flowStatus[internal.ErrorKey] = flow.PUKRetries
		finalType = flow.SwapCard
	} else {
		if mkf.insertedKeycard.PinRetries == 0 {
			if len(enteredPUK) == flow.DefPUKLen {
				if len(enteredPIN) == flow.DefPINLen && enteredPIN == enteredNewPIN {
					if enteredPUK != mkf.insertedKeycard.Puk {
						mkf.insertedKeycard.PukRetries--
						if mkf.insertedKeycard.PukRetries == 0 {
							flowStatus[internal.ErrorKey] = flow.PUKRetries
							finalType = flow.SwapCard
						} else {
							flowStatus[internal.ErrorKey] = flow.PUK
							finalType = flow.EnterPUK
						}
					}
				} else {
					flowStatus[internal.ErrorKey] = internal.ErrorUnblocking
					finalType = flow.EnterNewPIN
				}
			} else {
				flowStatus[internal.ErrorKey] = ""
				finalType = flow.EnterPUK
			}
		} else {
			if len(enteredNewPIN) == 0 && len(enteredPIN) == flow.DefPINLen && enteredPIN != mkf.insertedKeycard.Pin {
				mkf.insertedKeycard.PinRetries--
				flowStatus[internal.ErrorKey] = flow.PIN
				finalType = flow.EnterPIN
				if mkf.insertedKeycard.PinRetries == 0 {
					flowStatus[internal.ErrorKey] = ""
					finalType = flow.EnterPUK
				}
			}
		}
	}

	if mkf.insertedKeycard.PinRetries > 0 && len(enteredPIN) == flow.DefPINLen && enteredPIN == mkf.insertedKeycard.Pin ||
		mkf.insertedKeycard.PinRetries == 0 && mkf.insertedKeycard.PukRetries > 0 && len(enteredPUK) == flow.DefPUKLen &&
			enteredPUK == mkf.insertedKeycard.Puk && len(enteredPIN) == flow.DefPINLen && enteredPIN == enteredNewPIN {

		mkf.insertedKeycard.PinRetries = flow.MaxPINRetries
		mkf.insertedKeycard.PukRetries = flow.MaxPUKRetries
		mkf.insertedKeycard.Pin = enteredPIN

		if exportMaster {
			if mkf.insertedKeycardHelper.MasterKeyAddress == "" {
				iAsStr := strconv.Itoa(rand.Intn(100) + 100)
				mkf.insertedKeycardHelper.MasterKeyAddress = "0x" + strings.Repeat("0", 40-len(iAsStr)) + iAsStr
			}
			flowStatus[flow.MasterAddr] = mkf.insertedKeycardHelper.MasterKeyAddress
		}

		if path, ok := mkf.params[flow.BIP44Path]; ok {
			if mkf.insertedKeycardHelper.ExportedKey == nil {
				mkf.insertedKeycardHelper.ExportedKey = make(map[string]internal.KeyPair)
			}

			if pathStr, ok := path.(string); ok {
				keyPair, _ := mkf.insertedKeycardHelper.ExportedKey[pathStr]

				if keyPair.Address == "" {
					keyPair.Address = "0x" + strings.Repeat("0", 39) + "1"
				}

				if len(keyPair.PublicKey) == 0 {
					keyPair.PublicKey = []byte(strings.Repeat("0", 129) + "1")
				}

				if !exportPrivate {
					keyPair.PrivateKey = []byte("")
				} else if len(keyPair.PrivateKey) == 0 {
					keyPair.PrivateKey = []byte(strings.Repeat("0", 63) + "1")
				}

				mkf.insertedKeycardHelper.ExportedKey[pathStr] = keyPair
				flowStatus[flow.ExportedKey] = keyPair
			} else if paths, ok := path.([]interface{}); ok {
				keys := make([]*internal.KeyPair, len(paths))

				for i, path := range paths {
					keyPair, _ := mkf.insertedKeycardHelper.ExportedKey[path.(string)]

					if keyPair.Address == "" {
						iAsStr := strconv.Itoa(i + 1)
						keyPair.Address = "0x" + strings.Repeat("0", 40-len(iAsStr)) + iAsStr
					}

					if len(keyPair.PublicKey) == 0 {
						iAsStr := strconv.Itoa(i + 1)
						keyPair.PublicKey = []byte(strings.Repeat("0", 130-len(iAsStr)) + iAsStr)
					}

					if !exportPrivate {
						keyPair.PrivateKey = []byte("")
					} else if len(keyPair.PrivateKey) == 0 {
						iAsStr := strconv.Itoa(i + 1)
						keyPair.PrivateKey = []byte(strings.Repeat("0", 64-len(iAsStr)) + iAsStr)
					}

					mkf.insertedKeycardHelper.ExportedKey[path.(string)] = keyPair
					keys[i] = &keyPair
				}
				flowStatus[flow.ExportedKey] = keys
			}
		}

		mkf.state = flow.Idle
		signal.Send(flow.FlowResult, flowStatus)
		return
	}

	flowStatus[flow.FreeSlots] = mkf.insertedKeycard.FreePairingSlots
	flowStatus[flow.PINRetries] = mkf.insertedKeycard.PinRetries
	flowStatus[flow.PUKRetries] = mkf.insertedKeycard.PukRetries
	mkf.state = flow.Paused
	signal.Send(finalType, flowStatus)
}
