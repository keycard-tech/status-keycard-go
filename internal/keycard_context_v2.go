package internal

import (
	"github.com/ebfe/scard"
	"go.uber.org/zap"
	"runtime"
	"github.com/status-im/keycard-go/io"
	"github.com/status-im/keycard-go"
	"context"
	"github.com/status-im/status-keycard-go/pkg/pairing"
	"github.com/status-im/keycard-go/types"
	"github.com/pkg/errors"
	"github.com/status-im/status-keycard-go/signal"
	"sync"
)

const infiniteTimeout = -1
const zeroTimeout = 0

var pnpReader = scard.ReaderState{
	Reader:       `\\?PnP?\Notification`,
	CurrentState: scard.StateUnaware,
}

type KeycardContextV2 struct {
	KeycardContext

	shutdown     func()
	forceScan    bool // Needed to distinguish cardCtx.cancel() from a real shutdown
	logger       *zap.Logger
	pairings     *pairing.Store
	status       *Status
	readers      ReadersStates
	activeReader string
	activeCard   string
	mutex        *sync.RWMutex
	//readers      chan []string
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
		mutex:    &sync.RWMutex{},
	}

	err = kc.establishContext()
	if err != nil {
		kc.logger.Error("failed to establish context", zap.Error(err))
		return nil, err
	}

	err = kc.checkPnpFeature()
	if err != nil {
		kc.logger.Error("PnP notifications are not supported", zap.Error(err))
		return nil, err
	}

	ctx, cancel := context.WithCancel(context.Background())
	kc.shutdown = cancel
	kc.forceScan = false

	go kc.cardCommunicationRoutine(ctx)
	go kc.monitor()

	return kc, nil
}

func (kc *KeycardContextV2) checkPnpFeature() error {
	rs := []scard.ReaderState{pnpReader}
	err := kc.cardCtx.GetStatusChange(rs, 0)
	if err != nil {
		return errors.Wrap(err, "failed to get status change")
	}

	if rs[0].EventState&scard.StateUnknown == 0 {
		return nil
	}

	return errors.New("PnP feature not supported")
}

func (kc *KeycardContextV2) monitor() {
	if kc.cardCtx == nil {
		panic("card context is nil")
	}

	logger := kc.logger.Named("monitor")
	defer kc.logger.Debug("monitor stopped")

	// Add special reader for waiting for new readers
	// WARNING: Maybe need to ensure this feature is OS-supported:
	// 			https://blog.apdu.fr/posts/2015/12/os-x-el-capitan-missing-feature/

	for {
		kc.monitorRoutine(logger)
	}
}

func (kc *KeycardContextV2) monitorRoutine(logger *zap.Logger) {
	// Get current readers list and state
	readers, err := kc.getCurrentReadersState()
	if err != nil {
		logger.Error("failed to get readers state", zap.Error(err))
		return
	}

	logger.Debug("new readers list", zap.Any("available", readers))

	kc.mutex.Lock()
	kc.readers = readers
	kc.mutex.Unlock()

	err, _ = kc.scanReadersForKeycard(readers)
	if err != nil {
		logger.Error("failed to check readers", zap.Error(err))
		// FIXME: simply continue?
	}

	// Wait for readers changes, including new readers
	rs := append(readers, pnpReader)

	// Wait for reader changes
	err = kc.cardCtx.GetStatusChange(rs, infiniteTimeout)
	if err == scard.ErrCancelled && !kc.forceScan {
		// Shutdown requested
		return
	}
	if err != scard.ErrCancelled && err != nil {
		kc.logger.Error("failed to get status change", zap.Error(err))
		return
	}

	kc.logger.Debug("readers changed", zap.Any("readers", rs))
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

	if len(rs) == 0 {
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

func (kc *KeycardContextV2) scanReadersForKeycard(readers ReadersStates) (error, bool) {
	if !kc.forceScan &&
		kc.activeReader != "" &&
		readers.Contains(kc.activeReader) &&
		readers.ReaderHasCard(kc.activeReader) {
		// active reader is not selected yet or is still present
		// no need to connect card
		return nil, false
	}

	kc.forceScan = false
	kc.resetCardConnection(false)

	readerWithCardIndex, ok := readers.ReaderWithCardIndex()
	if !ok {
		kc.logger.Debug("no card found on any readers")
		return nil, false
	}

	kc.logger.Debug("card found", zap.Int("index", readerWithCardIndex))

	err := kc.connectCard(readers[readerWithCardIndex].Reader)
	if err != nil {
		kc.logger.Error("failed to connect card", zap.Error(err))
		//time.Sleep(500 * time.Millisecond)
		return err, false
	}

	kc.logger.Debug("card connected")

	err = kc.connectKeycard()
	if err != nil {
		kc.logger.Error("failed to connect keycard", zap.Error(err))
		return err, false
	}

	kc.logger.Info("keycard connected",
		zap.Any("appInfo", kc.status.AppInfo),
		zap.Any("appStatus", kc.status.AppStatus),
	)

	return nil, true
}

func (kc *KeycardContextV2) connectCard(reader string) error {
	card, err := kc.cardCtx.Connect(reader, scard.ShareShared, scard.ProtocolAny)
	if err != nil {
		return err
	}

	_, err = card.Status()
	if err != nil {
		return err
	}

	kc.activeReader = reader
	kc.card = card
	kc.c = io.NewNormalChannel(kc)
	kc.cmdSet = keycard.NewCommandSet(kc.c)

	return nil
}

func (kc *KeycardContextV2) connectKeycard() error {
	info, err := kc.SelectApplet()
	if err != nil {
		kc.logger.Error("failed to select applet", zap.Error(err))
		return err
	}

	//
	// NOTE: copy of openSC
	//

	if !info.Installed {
		return errors.New("not a keycard")
	}

	appInfo := ToAppInfo(info)

	pair := kc.pairings.Get(appInfo.InstanceUID.String())

	if pair == nil {
		kc.logger.Debug("pairing not found, pairing now")

		//
		// NOTE: copy of pair
		//
		var pairingInfo *types.PairingInfo
		pairingPassword := DefPairing
		pairingInfo, err = kc.Pair(pairingPassword)
		if err != nil {
			kc.logger.Error("failed to pair", zap.Error(err))
			//status.State = PairingError
			return errors.New("pairing error")
		}

		pair = pairing.ToPairInfo(pairingInfo)
		err = kc.pairings.Store(appInfo.InstanceUID.String(), pair)
		if err != nil {
			kc.logger.Error("failed to store pairing", zap.Error(err))
			//status.State = PairingError
			return errors.New("pairing error")
		}
	}

	err = kc.OpenSecureChannel(pair.Index, pair.Key)
	if err != nil {
		//logger.Error("failed to open secure channel", zap.Error(err))
		return errors.Wrap(err, "failed to open secure channel")
	}

	appStatus, err := kc.GetStatusApplication()
	if err != nil {
		kc.logger.Error("failed to get application status", zap.Error(err))
		return errors.Wrap(err, "failed to get application status")
	}

	kc.logger.Info("application status",
		zap.Stringer("instanceUID", appInfo.InstanceUID),
		zap.Any("appStatus", appStatus),
	)

	kc.status.AppInfo = &appInfo
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

func (kc *KeycardContextV2) Stop() {
	kc.forceScan = true
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

func (kc *KeycardContextV2) publishStatus() {
	//kc.logger.Debug("status changed", zap.Any("status", kc.status))
	signal.Send("status-changed", kc.status)
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

func (kc *KeycardContextV2) VerifyPIN(pin string) error {
	if kc.cmdSet == nil {
		return errors.New("keycard not connected")
	}

	err := kc.cmdSet.VerifyPIN(pin)
	if err == nil {
		return nil
	}

	if IsSCardError(err) {
		kc.logger.Error("failed to verify pin", zap.Error(err))
		kc.resetCardConnection(true)
	}

	return err
}
