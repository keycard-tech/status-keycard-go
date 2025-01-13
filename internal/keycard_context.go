package internal

import (
	"runtime"
	"time"

	"github.com/pkg/errors"
	"github.com/ebfe/scard"
	keycard "github.com/status-im/keycard-go"
	"github.com/status-im/keycard-go/io"
	"github.com/status-im/keycard-go/types"
)

const bip39Salt = "mnemonic"

type commandType int

const (
	Close commandType = iota
	Transmit
	Ack
)

type KeycardContext struct {
	cardCtx   *scard.Context
	card      *scard.Card
	readers   []string
	c         types.Channel
	cmdSet    *keycard.CommandSet
	connected chan (bool)
	command   chan (commandType)
	apdu      []byte
	rpdu      []byte
	runErr    error
}

// Transmit implements the Channel and Transmitter interfaces
// KeycardContext is used to
// https://github.com/keycard-tech/status-keycard-go/blob/ff46592beb81308e49ae63763c0efc63473692f6/internal/keycard_context.go#L192-L193
func (kc *KeycardContext) Transmit(apdu []byte) ([]byte, error) {
	kc.apdu = apdu
	kc.command <- Transmit
	<-kc.command
	kc.apdu = nil
	rpdu, err := kc.rpdu, kc.runErr
	kc.rpdu = nil
	kc.runErr = nil
	return rpdu, err
}

func StartKeycardContext(filepath string) (*KeycardContext, error) {
	kc := &KeycardContext{
		connected: make(chan (bool)),
		command:   make(chan (commandType)),
	}

	err := kc.establishContext()
	if err != nil {
		return nil, err
	}

	go kc.run()
	<-kc.connected

	if kc.runErr != nil {
		return nil, kc.runErr
	}

	return kc, nil
}

func (kc *KeycardContext) Stop() {
	close(kc.command)
}

func (kc *KeycardContext) establishContext() error {
	cardCtx, err := scard.EstablishContext()
	if err != nil {
		return errors.New(ErrorPCSC)
	}

	kc.cardCtx = cardCtx
	return nil
}

func (kc *KeycardContext) run() {
	runtime.LockOSThread()

	var err error

	defer func() {
		if err != nil {
			Printf(err.Error())
		}

		kc.runErr = err

		if kc.cardCtx != nil {
			_ = kc.cardCtx.Release()
		}

		close(kc.connected)
		runtime.UnlockOSThread()
	}()

	err = kc.start()

	if err != nil {
		return
	}

	kc.connected <- true

	err = kc.connect()

	if err != nil {
		return
	}

	kc.connected <- true

	for cmd := range kc.command {
		switch cmd {
		case Transmit:
			kc.rpdu, kc.runErr = kc.card.Transmit(kc.apdu)
			kc.command <- Ack
		case Close:
			return
		}
	}
}

func (kc *KeycardContext) start() error {
	cardCtx, err := scard.EstablishContext()
	if err != nil {
		return errors.New(ErrorPCSC)
	}

	Printf("listing readers")
	readers, err := cardCtx.ListReaders()
	if err != nil {
		return errors.New(ErrorReaderList)
	}

	kc.readers = readers

	if len(readers) == 0 {
		return errors.New(ErrorNoReader)
	}

	kc.cardCtx = cardCtx
	return nil
}

func (kc *KeycardContext) connect() error {
	Printf("waiting for card")
	index, err := kc.waitForCard(kc.cardCtx, kc.readers)
	if err != nil {
		return err
	}

	Printf("card found at index %d", index)
	reader := kc.readers[index]

	Printf("using reader %s", reader)

	card, err := kc.cardCtx.Connect(reader, scard.ShareShared, scard.ProtocolAny)
	if err != nil {
		// error connecting to card
		time.Sleep(500 * time.Millisecond)
		return err
	}

	status, err := card.Status()
	if err != nil {
		time.Sleep(500 * time.Millisecond)
		return err
	}

	switch status.ActiveProtocol {
	case scard.ProtocolT0:
		Printf("card protocol T0")
	case scard.ProtocolT1:
		Printf("card protocol T1")
	default:
		Printf("card protocol T unknown")
	}

	kc.card = card
	kc.c = io.NewNormalChannel(kc)
	kc.cmdSet = keycard.NewCommandSet(kc.c)

	return nil
}

func (kc *KeycardContext) waitForCard(ctx *scard.Context, readers []string) (int, error) {
	rs := make([]scard.ReaderState, len(readers))

	for i := range rs {
		rs[i].Reader = readers[i]
		rs[i].CurrentState = scard.StateUnaware
	}

	for {
		for i := range rs {
			if rs[i].EventState&scard.StatePresent != 0 {
				return i, nil
			}

			rs[i].CurrentState = rs[i].EventState
		}

		err := ctx.GetStatusChange(rs, -1)
		if err != nil {
			return -1, err
		}
	}
}

func (kc *KeycardContext) PairingInfo() *types.PairingInfo {
	return kc.cmdSet.PairingInfo
}

func (kc *KeycardContext) ApplicationInfo() *types.ApplicationInfo {
	return kc.cmdSet.ApplicationInfo
}

func (kc *KeycardContext) Connected() <-chan bool {
	return kc.connected
}

func (kc *KeycardContext) RunErr() error {
	return kc.runErr
}
