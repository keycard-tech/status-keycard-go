package internal

import (
	"context"
	"runtime"

	"github.com/ebfe/scard"
	"github.com/pkg/errors"
	"github.com/status-im/keycard-go"
	"github.com/status-im/keycard-go/io"
	"github.com/status-im/keycard-go/types"
	"go.uber.org/zap"

	"github.com/status-im/status-keycard-go/pkg/pairing"
	"github.com/status-im/status-keycard-go/signal"
)

const infiniteTimeout = -1
const zeroTimeout = 0
const pnpNotificationReader = `\\?PnP?\Notification`

var (
	errKeycardNotConnected = errors.New("keycard not connected")
)

type KeycardContextV2 struct {
	KeycardContext

	shutdown     func()
	forceScan    bool // Needed to distinguish cardCtx.cancel() from a real shutdown
	logger       *zap.Logger
	pairings     *pairing.Store
	status       *Status
	activeReader string
}

func NewKeycardContextV2(pairingsStoreFilePath string) (*KeycardContextV2, error) {
	pairingsStore, err := pairing.NewStore(pairingsStoreFilePath)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create pairing store")
	}

	kc := &KeycardContextV2{
		KeycardContext: KeycardContext{
			command: make(chan commandType),
		},
		logger:   zap.L().Named("keycard"),
		pairings: pairingsStore,
		status:   NewStatus(),
	}

	err = kc.establishContext()
	if err != nil {
		kc.logger.Error("failed to establish context", zap.Error(err))
		kc.status.State = NoPCSC
		kc.publishStatus()
		return nil, err
	}

	ctx, cancel := context.WithCancel(context.Background())
	kc.shutdown = cancel
	kc.forceScan = false

	go kc.cardCommunicationRoutine(ctx)
	kc.monitor()

	return kc, nil
}

func (kc *KeycardContext) establishContext() error {
	cardCtx, err := scard.EstablishContext()
	if err != nil {
		return errors.New(ErrorPCSC)
	}

	kc.cardCtx = cardCtx
	return nil
}

func (kc *KeycardContext) cardCommunicationRoutine(ctx context.Context) {
	// Communication with the keycard must be done in a fixed thread
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	for {
		select {
		case <-ctx.Done():
			return
		case cmd := <-kc.command:
			switch cmd {
			case Transmit:
				kc.rpdu, kc.runErr = kc.card.Transmit(kc.apdu)
				kc.command <- Ack
			case Close:
				return
			default:
				break
			}
		}
	}
}

func (kc *KeycardContextV2) monitor() {
	if kc.cardCtx == nil {
		panic("card context is nil")
	}

	logger := kc.logger.Named("monitor")

	go func() {
		defer logger.Debug("monitor stopped")
		// This goroutine will be stopped by cardCtx.Cancel()
		for {
			finish := kc.monitorRoutine(logger)
			if finish {
				return
			}
		}
	}()
}

func (kc *KeycardContextV2) monitorRoutine(logger *zap.Logger) bool {
	// Get current readers list and state
	readers, err := kc.getCurrentReadersState()
	if err != nil {
		logger.Error("failed to get readers state", zap.Error(err))
		kc.status.Reset()
		kc.status.State = InternalError
		kc.publishStatus()
		return false
	}

	logger.Debug("readers list updated", zap.Any("available", readers))

	if readers.Empty() {
		kc.status.State = WaitingForReader
		kc.status.AppInfo = nil
		kc.status.AppStatus = nil
		kc.publishStatus()
	}

	err = kc.scanReadersForKeycard(readers)
	if err != nil {
		logger.Error("failed to check readers", zap.Error(err))
	}

	// Wait for readers changes, including new readers
	// https://blog.apdu.fr/posts/2024/08/improved-scardgetstatuschange-for-pnpnotification-special-reader/
	// NOTE: The article states that MacOS is not supported, but works for me on MacOS 15.1.1 (24B91).
	pnpReader := scard.ReaderState{
		Reader:       pnpNotificationReader,
		CurrentState: scard.StateUnaware,
	}
	rs := append(readers, pnpReader)

	err = kc.cardCtx.GetStatusChange(rs, infiniteTimeout)
	if err == scard.ErrCancelled && !kc.forceScan {
		// Shutdown requested
		return true
	}
	if err != scard.ErrCancelled && err != nil {
		kc.logger.Error("failed to get status change", zap.Error(err))
		return false
	}

	return false
}

func (kc *KeycardContextV2) getCurrentReadersState() (ReadersStates, error) {
	readers, err := kc.cardCtx.ListReaders()
	if err != nil {
		return nil, err
	}

	rs := make(ReadersStates, len(readers))
	for i, name := range readers {
		rs[i].Reader = name
		rs[i].CurrentState = scard.StateUnaware
	}

	if rs.Empty() {
		return rs, nil
	}

	err = kc.cardCtx.GetStatusChange(rs, zeroTimeout)
	if err != nil {
		return nil, err
	}

	rs.Update()

	// When removing a reader, a call to `ListReaders` too quick might still return the removed reader.
	// So we need to filter out the unknown readers.
	knownReaders := make(ReadersStates, 0, len(rs))
	for i := range rs {
		if rs[i].EventState&scard.StateUnknown == 0 {
			knownReaders.Append(rs[i])
		}
	}

	return knownReaders, nil
}

func (kc *KeycardContextV2) scanReadersForKeycard(readers ReadersStates) error {
	if !kc.forceScan &&
		kc.activeReader != "" &&
		readers.Contains(kc.activeReader) &&
		readers.ReaderHasCard(kc.activeReader) {
		// active reader is not selected yet or is still present, no need to connect a card
		return nil
	}

	if readers.Empty() {
		return nil
	}

	kc.forceScan = false
	kc.resetCardConnection(false)

	readerWithCardIndex, ok := readers.ReaderWithCardIndex()
	if !ok {
		kc.logger.Debug("no card found on any readers")
		kc.status.State = WaitingForCard
		kc.status.AppInfo = nil
		kc.status.AppStatus = nil
		kc.publishStatus()
		return nil
	}

	kc.logger.Debug("card found", zap.Int("index", readerWithCardIndex))

	err := kc.connectKeycard(readers[readerWithCardIndex].Reader)
	if err != nil {
		kc.logger.Error("failed to connect keycard", zap.Error(err))
		kc.publishStatus()
		return err
	}

	kc.publishStatus()
	return nil
}

func (kc *KeycardContextV2) connectKeycard(reader string) error {
	card, err := kc.cardCtx.Connect(reader, scard.ShareShared, scard.ProtocolAny)
	if err != nil {
		kc.status.State = ConnectionError
		return errors.Wrap(err, "failed to connect to card")
	}

	_, err = card.Status()
	if err != nil {
		kc.status.State = ConnectionError
		return errors.Wrap(err, "failed to get card status")
	}

	kc.activeReader = reader
	kc.card = card
	kc.c = io.NewNormalChannel(kc)
	kc.cmdSet = keycard.NewCommandSet(kc.c)

	// Card connected, now check if this is a keycard

	info, err := kc.SelectApplet()
	if err != nil {
		kc.status.State = ConnectionError
		return errors.Wrap(err, "failed to select applet")
	}

	//
	// NOTE: copy of openSC
	//

	appInfo := ToAppInfoV2(info)
	kc.status.AppInfo = &appInfo

	if !info.Installed {
		kc.status.State = NotKeycard
		return errors.New("card is not a keycard")
	}

	if !info.Initialized {
		kc.status.State = EmptyKeycard
		return errors.New("keycard not initialized")
	}

	kc.status.State = ConnectingCard
	kc.publishStatus()

	pair := kc.pairings.Get(appInfo.InstanceUID.String())

	if pair == nil {
		kc.logger.Debug("pairing not found, pairing now")

		//
		// NOTE: copy of pair
		//
		var pairingInfo *types.PairingInfo
		pairingPassword := DefPairing
		pairingInfo, err = kc.Pair(pairingPassword)
		if errors.Is(err, keycard.ErrNoAvailablePairingSlots) {
			kc.status.State = NoAvailablePairingSlots
			return err
		}

		if err != nil {
			kc.status.State = PairingError
			return errors.Wrap(err, "failed to pair keycard")
		}

		pair = pairing.ToPairInfo(pairingInfo)
		err = kc.pairings.Store(appInfo.InstanceUID.String(), pair)
		if err != nil {
			kc.status.State = InternalError
			return errors.Wrap(err, "failed to store pairing")
		}
	}

	err = kc.OpenSecureChannel(pair.Index, pair.Key)
	if err != nil {
		kc.status.State = ConnectionError
		return errors.Wrap(err, "failed to open secure channel")
	}

	appStatus, err := kc.GetStatusApplication()
	if err != nil {
		kc.status.State = ConnectionError
		return errors.Wrap(err, "failed to get application status")
	}

	kc.status.State = Ready
	kc.status.AppStatus = appStatus

	return nil
}

func (kc *KeycardContextV2) resetCardConnection(forceRescan bool) {
	kc.logger.Debug("reset card connection")
	kc.activeReader = ""
	kc.card = nil
	kc.c = nil
	kc.cmdSet = nil

	// If a command failed, we need to cancel the context. This will force the monitor to reconnect to the card.
	if forceRescan {
		kc.forceScan = true
		err := kc.cardCtx.Cancel()
		if err != nil {
			kc.logger.Error("failed to cancel context", zap.Error(err))
		}
	}
}

func (kc *KeycardContextV2) publishStatus() {
	kc.logger.Info("status changed", zap.Any("status", kc.status))
	signal.Send("status-changed", kc.status)
}

func (kc *KeycardContextV2) Stop() {
	kc.forceScan = false
	if kc.cardCtx != nil {
		err := kc.cardCtx.Cancel()
		if err != nil {
			kc.logger.Error("failed to cancel context", zap.Error(err))
		}
	}
	kc.KeycardContext.Stop()
	if kc.shutdown != nil {
		kc.shutdown()
	}
}

func (kc *KeycardContextV2) keycardConnected() bool {
	return kc.cmdSet != nil
}

func (kc *KeycardContextV2) checkSCardError(err error, context string) error {
	if err == nil {
		return nil
	}

	if IsSCardError(err) {
		kc.logger.Error("command failed, resetting connection",
			zap.String("context", context),
			zap.Error(err))
		kc.resetCardConnection(true)
	}

	return err
}

func (kc *KeycardContextV2) GetStatus() Status {
	return *kc.status
}

func (kc *KeycardContextV2) Initialize(pin, puk, pairingPassword string) error {
	if !kc.keycardConnected() {
		return errKeycardNotConnected
	}

	secrets := keycard.NewSecrets(pin, puk, pairingPassword)
	err := kc.cmdSet.Init(secrets)
	if err != nil {
		return kc.checkSCardError(err, "Init")
	}

	// Reset card connection to pair the card and open secure channel
	kc.resetCardConnection(true)
	return nil
}

func (kc *KeycardContextV2) VerifyPIN(pin string) error {
	if !kc.keycardConnected() {
		return errKeycardNotConnected
	}

	err := kc.cmdSet.VerifyPIN(pin)
	return kc.checkSCardError(err, "VerifyPIN")
}

func (kc *KeycardContextV2) GenerateMnemonic(mnemonicLength int) ([]int, error) {
	if !kc.keycardConnected() {
		return nil, errKeycardNotConnected
	}

	indexes, err := kc.cmdSet.GenerateMnemonic(mnemonicLength / 3)
	return indexes, kc.checkSCardError(err, "GenerateMnemonic")
}

func (kc *KeycardContextV2) LoadMnemonic(mnemonic string, password string) ([]byte, error) {
	if !kc.keycardConnected() {
		return nil, errKeycardNotConnected
	}

	seed := kc.mnemonicToBinarySeed(mnemonic, password)
	keyUID, err := kc.loadSeed(seed)
	return keyUID, kc.checkSCardError(err, "LoadMnemonic")
}

func (kc *KeycardContextV2) FactoryReset() error {
	if !kc.keycardConnected() {
		return errKeycardNotConnected
	}

	kc.status.Reset()
	kc.status.State = FactoryResetting
	kc.publishStatus()

	kc.logger.Debug("factory reset")
	err := kc.KeycardContext.FactoryReset(true)

	// Reset card connection to read the card data
	kc.resetCardConnection(true)
	return err
}
