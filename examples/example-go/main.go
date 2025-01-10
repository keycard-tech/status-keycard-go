package main

import "C"

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/status-im/status-keycard-go/signal"
	"github.com/status-im/status-keycard-go/pkg/flow"
)

var currentFlow *flow.KeycardFlow
var finished chan (struct{})
var correctPairing = "KeycardDefaultPairing"
var correctPIN = "123456"
var correctPUK = "123456123456"
var keyUID = "136cbfc087cf7df6cf3248bce7563d4253b302b2f9e2b5eef8713fa5091409bc"

func signalHandler(j []byte) {
	var sig signal.Envelope
	json.Unmarshal(j, &sig)
	fmt.Printf("Received signal: %+v\n", sig)

	go func() {
		switch sig.Type {
		case flow.InsertCard:
			fmt.Print("Insert card\n")
		case flow.CardInserted:
			fmt.Printf("Card inserted\n")
		case flow.SwapCard:
			fmt.Printf("Swap card. Changing constraint\n")
			currentFlow.Resume(flow.FlowParams{flow.KeyUID: keyUID})
		case flow.EnterPairing:
			fmt.Printf("Entering pass: %+v\n", correctPairing)
			currentFlow.Resume(flow.FlowParams{flow.PairingPass: correctPairing})
		case flow.EnterPIN:
			fmt.Printf("Entering PIN: %+v\n", correctPIN)
			currentFlow.Resume(flow.FlowParams{flow.PIN: correctPIN})
		case flow.EnterNewPIN:
			fmt.Printf("Creating PIN: %+v\n", correctPIN)
			currentFlow.Resume(flow.FlowParams{flow.NewPIN: correctPIN})
		case flow.EnterNewPUK:
			fmt.Printf("Creating PUK: %+v\n", correctPUK)
			currentFlow.Resume(flow.FlowParams{flow.NewPUK: correctPUK})
		case flow.EnterNewPair:
			fmt.Printf("Creating pairing: %+v\n", correctPairing)
			currentFlow.Resume(flow.FlowParams{flow.NewPairing: correctPairing})
		case flow.EnterMnemonic:
			fmt.Printf("Loading mnemonic\n")
			currentFlow.Resume(flow.FlowParams{flow.Mnemonic: "receive fan copper bracket end train again sustain wet siren throw cigar"})
		case flow.FlowResult:
			fmt.Printf("Flow result: %+v\n", sig.Event)
			close(finished)
		}
	}()
}

func testFlow(typ flow.FlowType, params flow.FlowParams) {
	finished = make(chan struct{})
	err := currentFlow.Start(typ, params)

	if err != nil {
		fmt.Printf("error: %+v\n", err)
	}

	<-finished
}

func testRecoverAccount() {
	finished = make(chan struct{})
	err := currentFlow.Start(flow.RecoverAccount, flow.FlowParams{})

	if err != nil {
		fmt.Printf("error: %+v\n", err)
	}

	<-finished
}

func main() {
	dir, err := os.MkdirTemp("", "status-keycard-go")
	if err != nil {
		fmt.Printf("error: %+v\n", err)
		return
	}

	defer os.RemoveAll(dir)

	pairingsFile := filepath.Join(dir, "keycard-pairings.json")

	currentFlow, err = flow.NewFlow(pairingsFile)

	if err != nil {
		fmt.Printf("error: %+v\n", err)
		return
	}

	signal.SetKeycardSignalHandler(signalHandler)

	testFlow(flow.GetAppInfo, flow.FlowParams{flow.FactoryReset: true})
	testFlow(flow.LoadAccount, flow.FlowParams{flow.MnemonicLen: 12})
	testFlow(flow.UnpairThis, flow.FlowParams{flow.PIN: correctPIN})
	testFlow(flow.RecoverAccount, flow.FlowParams{flow.PairingPass: "WrongPass", flow.PIN: "234567"})
	testFlow(flow.Login, flow.FlowParams{flow.KeyUID: "60a78c98d5dd659f714eb7072bfb2c0d8a65f74a8f6aff7bb27cf56ae1feec17"})
	testFlow(flow.GetAppInfo, flow.FlowParams{})
	testFlow(flow.ExportPublic, flow.FlowParams{flow.BIP44Path: "m/44'/60'/0'/0/1", flow.ExportMaster: true})
	testFlow(flow.ExportPublic, flow.FlowParams{flow.BIP44Path: "m/43'/60'/1581'/1'/0", flow.ExportPriv: true})
	testFlow(flow.ExportPublic, flow.FlowParams{flow.BIP44Path: []interface{}{"m/44'/60'/0'/0/2", "m/44'/60'/0'/0/3", "m/44'/60'/0'/0/4"}})
	testFlow(flow.Sign, flow.FlowParams{flow.TXHash: "60a78c98d5dd659f714eb7072bfb2c0d8a65f74a8f6aff7bb27cf56ae1feec17", flow.BIP44Path: "m/44'/60'/0'/0/0"})
	testFlow(flow.StoreMetadata, flow.FlowParams{flow.CardName: "TestCard", flow.WalletPaths: []interface{}{"m/44'/60'/0'/0/0", "m/44'/60'/0'/0/1", "m/44'/60'/0'/0/5", "m/44'/60'/0'/0/6"}})
	testFlow(flow.GetMetadata, flow.FlowParams{})
	testFlow(flow.GetMetadata, flow.FlowParams{flow.ResolveAddr: true})
	testFlow(flow.UnpairThis, flow.FlowParams{flow.PIN: correctPIN})
}
