package flow

import (
	"strconv"
	"strings"

	"github.com/status-im/status-keycard-go/signal"
	"github.com/status-im/status-keycard-go/internal"
)

func (mkf *MockedKeycardFlow) handleStoreMetadataFlow() {
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

	finalType := FlowResult
	flowStatus = FlowStatus{
		InstanceUID: mkf.insertedKeycard.InstanceUID,
		KeyUID:      mkf.insertedKeycard.KeyUID,
	}

	var (
		enteredPIN      string
		enteredCardName string
	)

	if v, ok := mkf.params[PIN]; ok {
		enteredPIN = v.(string)
	}
	if v, ok := mkf.params[CardName]; ok {
		enteredCardName = v.(string)
	}

	if len(enteredPIN) == defPINLen && enteredPIN == mkf.insertedKeycard.Pin && enteredCardName != "" {
		mkf.insertedKeycard.Metadata.Name = enteredCardName
		mkf.insertedKeycard.Metadata.Wallets = []internal.Wallet{}

		if v, ok := mkf.params[WalletPaths]; ok {
			wallets := v.([]interface{})

			for i, p := range wallets {
				if !strings.HasPrefix(p.(string), walletRoothPath) {
					panic("path must start with " + walletRoothPath)
				}

				tmpWallet := internal.Wallet{
					Path: p.(string),
				}

				found := false
				for _, w := range mkf.insertedKeycardHelper.Metadata.Wallets {
					if w.Path == tmpWallet.Path {
						found = true
						tmpWallet = w
						break
					}
				}

				if !found {
					iAsStr := strconv.Itoa(i + 1)
					tmpWallet.Address = "0x" + strings.Repeat("0", 40-len(iAsStr)) + iAsStr
					tmpWallet.PublicKey = []byte(strings.Repeat("0", 130-len(iAsStr)) + iAsStr)
					mkf.insertedKeycardHelper.Metadata.Wallets = append(mkf.insertedKeycardHelper.Metadata.Wallets, tmpWallet)
				}

				mkf.insertedKeycard.Metadata.Wallets = append(mkf.insertedKeycard.Metadata.Wallets, tmpWallet)
			}
		}

		mkf.state = Idle
		signal.Send(finalType, flowStatus)

		return
	}

	if len(enteredPIN) != defPINLen || enteredPIN != mkf.insertedKeycard.Pin {
		finalType = EnterPIN
	} else if enteredCardName == "" {
		finalType = EnterName
	}

	flowStatus[FreeSlots] = mkf.insertedKeycard.FreePairingSlots
	flowStatus[PINRetries] = mkf.insertedKeycard.PinRetries
	flowStatus[PUKRetries] = mkf.insertedKeycard.PukRetries
	mkf.state = Paused
	signal.Send(finalType, flowStatus)
}
