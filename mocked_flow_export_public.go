package statuskeycardgo

import (
	"math/rand"
	"strconv"
	"strings"

	"github.com/status-im/status-keycard-go/signal"
	"github.com/status-im/status-keycard-go/internal"
)

func (mkf *MockedKeycardFlow) handleExportPublicFlow() {
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

	flowStatus = FlowStatus{
		InstanceUID: mkf.insertedKeycard.InstanceUID,
		KeyUID:      mkf.insertedKeycard.KeyUID,
	}

	if mkf.insertedKeycard.InstanceUID == "" || mkf.insertedKeycard.KeyUID == "" {
		flowStatus[internal.ErrorKey] = internal.ErrorNoKeys
		flowStatus[FreeSlots] = 0
		mkf.state = Paused
		signal.Send(SwapCard, flowStatus)
		return
	}

	var (
		enteredPIN    string
		enteredNewPIN string
		enteredPUK    string
		exportMaster  bool
		exportPrivate bool
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
	if v, ok := mkf.params[ExportMaster]; ok {
		exportMaster = v.(bool)
	}
	if v, ok := mkf.params[ExportPriv]; ok {
		exportPrivate = v.(bool)
	}

	finalType := EnterPIN
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

		if exportMaster {
			if mkf.insertedKeycardHelper.MasterKeyAddress == "" {
				iAsStr := strconv.Itoa(rand.Intn(100) + 100)
				mkf.insertedKeycardHelper.MasterKeyAddress = "0x" + strings.Repeat("0", 40-len(iAsStr)) + iAsStr
			}
			flowStatus[MasterAddr] = mkf.insertedKeycardHelper.MasterKeyAddress
		}

		if path, ok := mkf.params[BIP44Path]; ok {
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
				flowStatus[ExportedKey] = keyPair
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
				flowStatus[ExportedKey] = keys
			}
		}

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
