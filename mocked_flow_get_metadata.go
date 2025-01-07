package statuskeycardgo

import (
	"math/rand"
	"strconv"
	"strings"

	"github.com/status-im/status-keycard-go/signal"
	"github.com/status-im/status-keycard-go/internal"
)

func (mkf *MockedKeycardFlow) handleGetMetadataFlow() {
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

	if mkf.insertedKeycard.InstanceUID == "" || mkf.insertedKeycard.KeyUID == "" {
		mkf.state = Idle
		signal.Send(FlowResult, FlowStatus{internal.ErrorKey: internal.ErrorNoKeys})
		return
	}

	flowStatus = FlowStatus{
		InstanceUID: mkf.insertedKeycard.InstanceUID,
		KeyUID:      mkf.insertedKeycard.KeyUID,
	}

	if resolveAddr, ok := mkf.params[ResolveAddr]; ok && resolveAddr.(bool) {
		if mkf.insertedKeycard.FreePairingSlots == 0 {
			flowStatus[internal.ErrorKey] = FreeSlots
			flowStatus[FreeSlots] = mkf.insertedKeycard.FreePairingSlots
			mkf.state = Paused
			signal.Send(SwapCard, flowStatus)
			return
		}

		var (
			enteredPIN    string
			enteredNewPIN string
			enteredPUK    string
			exportMaster  bool
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

			if exportMaster {
				if mkf.insertedKeycardHelper.MasterKeyAddress == "" {
					iAsStr := strconv.Itoa(rand.Intn(100) + 100)
					mkf.insertedKeycardHelper.MasterKeyAddress = "0x" + strings.Repeat("0", 40-len(iAsStr)) + iAsStr
				}
				flowStatus[MasterAddr] = mkf.insertedKeycardHelper.MasterKeyAddress
			}

			mkf.insertedKeycard.PinRetries = maxPINRetries
			mkf.insertedKeycard.PukRetries = maxPUKRetries
			mkf.insertedKeycard.Pin = enteredPIN
			flowStatus[internal.ErrorKey] = ""
			flowStatus[CardMeta] = mkf.insertedKeycard.Metadata
			mkf.state = Idle
			signal.Send(FlowResult, flowStatus)
			return
		}

		flowStatus[FreeSlots] = mkf.insertedKeycard.FreePairingSlots
		flowStatus[PINRetries] = mkf.insertedKeycard.PinRetries
		flowStatus[PUKRetries] = mkf.insertedKeycard.PukRetries
		mkf.state = Paused
		signal.Send(finalType, flowStatus)
		return
	}

	pubMetadata := internal.Metadata{
		Name: mkf.insertedKeycard.Metadata.Name,
	}
	for _, m := range mkf.insertedKeycard.Metadata.Wallets {
		pubMetadata.Wallets = append(pubMetadata.Wallets, internal.Wallet{
			Path: m.Path,
		})
	}

	flowStatus[CardMeta] = pubMetadata
	mkf.state = Idle
	signal.Send(FlowResult, flowStatus)
}
