package mocked

import (
	"strconv"
	"strings"

	"github.com/status-im/status-keycard-go/internal"
	"github.com/status-im/status-keycard-go/pkg/flow"
	"github.com/status-im/status-keycard-go/signal"
)

func (mkf *MockedKeycardFlow) handleStoreMetadataFlow() {
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

	finalType := flow.FlowResult
	flowStatus = flow.FlowStatus{
		flow.InstanceUID: mkf.insertedKeycard.InstanceUID,
		flow.KeyUID:      mkf.insertedKeycard.KeyUID,
	}

	var (
		enteredPIN      string
		enteredCardName string
	)

	if v, ok := mkf.params[flow.PIN]; ok {
		enteredPIN = v.(string)
	}
	if v, ok := mkf.params[flow.CardName]; ok {
		enteredCardName = v.(string)
	}

	if len(enteredPIN) == internal.DefPINLen && enteredPIN == mkf.insertedKeycard.Pin && enteredCardName != "" {
		mkf.insertedKeycard.Metadata.Name = enteredCardName
		mkf.insertedKeycard.Metadata.Wallets = []internal.Wallet{}

		if v, ok := mkf.params[flow.WalletPaths]; ok {
			wallets := v.([]interface{})

			for i, p := range wallets {
				if !strings.HasPrefix(p.(string), internal.WalletRoothPath) {
					panic("path must start with " + internal.WalletRoothPath)
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

		mkf.state = flow.Idle
		signal.Send(finalType, flowStatus)

		return
	}

	if len(enteredPIN) != internal.DefPINLen || enteredPIN != mkf.insertedKeycard.Pin {
		finalType = flow.EnterPIN
	} else if enteredCardName == "" {
		finalType = flow.EnterName
	}

	flowStatus[flow.FreeSlots] = mkf.insertedKeycard.FreePairingSlots
	flowStatus[flow.PINRetries] = mkf.insertedKeycard.PinRetries
	flowStatus[flow.PUKRetries] = mkf.insertedKeycard.PukRetries
	mkf.state = flow.Paused
	signal.Send(finalType, flowStatus)
}
