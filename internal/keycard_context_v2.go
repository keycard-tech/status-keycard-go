package internal

import (
	"time"
	"github.com/ebfe/scard"
	"fmt"
	"go.uber.org/zap"
	"runtime"
	"github.com/status-im/keycard-go/io"
	"github.com/status-im/keycard-go"
	"context"
	"github.com/status-im/status-keycard-go/pkg/pairing"
	"github.com/status-im/keycard-go/types"
	"github.com/pkg/errors"
	"github.com/status-im/status-keycard-go/signal"
	"reflect"
)

type KeycardContextV2 struct {
	KeycardContext

	logger   *zap.Logger
	pairings *pairing.Store
	shutdown func()
	status   *Status
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
		return nil, err
	}

	ctx, cancel := context.WithCancel(context.Background())
	kc.shutdown = cancel

	go kc.monitor(ctx)
	go kc.cardCommunicationRoutine(ctx)

	return kc, nil
}

func (kc *KeycardContextV2) Stop() {
	kc.KeycardContext.Stop()
	if kc.shutdown != nil {
		kc.shutdown()
	}
}

func (kc *KeycardContextV2) publishStatus() {
	kc.logger.Debug("status changed", zap.Any("status", kc.status))
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

func (kc *KeycardContextV2) monitor(ctx context.Context) {
	defer kc.logger.Debug("monitor stopped")

	interval := 100 * time.Millisecond // Trigger first check immediately

	for {
		select {
		case <-ctx.Done():
			return
		case <-time.After(interval):
			interval = 5 * time.Second // Switch to 5s interval after first check
			kc.monitorRoutine()
		}
	}
}

func (kc *KeycardContextV2) monitorRoutine() {
	if kc.cardCtx == nil {
		panic("card context is nil")
	}

	logger := kc.logger.Named("monitor")
	logger.Debug("monitor routine tick")

	status := *kc.status
	defer func() {
		if !reflect.DeepEqual(*kc.status, status) {
			kc.publishStatus()
		}
	}()

	//
	// NOTE: copy of start
	//

	// FIXME: This should be only done once, unless the card context is lost.
	// 		  Use GetStatusChange right away if the card context already exist.

	// FIXME: To wait for reader, a special reader name can be used: "\\?PnP?\Notification"
	// 		  This will wait for a reader to be inserted or removed.
	//		  And no need for loop then.

	// GetStatusChange can be cancelled with `cardCtx.   cancel()`

	readers, err := kc.cardCtx.ListReaders()
	if err != nil {
		status.Reset()
		status.State = NoPCSC
		logger.Error("failed to list readers", zap.Error(err))
		// WARNING: Makes no sense to continue without PCSC
		return
	}

	if len(readers) == 0 {
		//log(internal.ErrorNoReader)
		status.Reset()
		status.State = NoReader
		return
	}

	if !reflect.DeepEqual(status.Readers, readers) {
		logger.Debug("readers changed")
		for i := range readers {
			logger.Debug(fmt.Sprintf("reader %d", i), zap.Any("reader", readers[i]))
		}
	}

	status.Readers = readers

	//
	// NOTE: copy of waitForCard
	//

	rs := make([]scard.ReaderState, len(readers))

	for i := range rs {
		rs[i].Reader = readers[i]
		rs[i].CurrentState = scard.StateUnaware
	}

	err = kc.cardCtx.GetStatusChange(rs, -1)
	if err != nil {
		logger.Error("failed to get status change", zap.Error(err))
		return
	}

	index := -1

	for i := range rs {
		if rs[i].EventState&scard.StatePresent == 0 {
			continue
		}

		// NOTE: For now we only support one card at a time
		index = i
		break
	}

	if index == -1 {
		status.State = NoCard
		status.CardInserted = false
		return
	}

	status.CardInserted = true

	//
	// NOTE: copy of connect
	//

	reader := readers[index]

	card, err := kc.cardCtx.Connect(reader, scard.ShareShared, scard.ProtocolAny)
	if err != nil {
		status.State = ConnectionError
		logger.Error("failed to connect to card", zap.Error(err))
		// WARNING: Does it make sense to continue the monitor loop?
		//time.Sleep(500 * time.Millisecond)
		return
	}

	_, err = card.Status()
	if err != nil {
		status.State = ConnectionError
		logger.Error("failed to get card status", zap.Error(err))
		// WARNING: Does it make sense to continue the monitor loop?
		//time.Sleep(500 * time.Millisecond)
		return
	}

	kc.card = card
	kc.c = io.NewNormalChannel(kc)
	kc.cmdSet = keycard.NewCommandSet(kc.c)

	// FIXME: defer reset card, channel and cmdSet

	// FIXME: No need to reopen the secure channel, if the card didn't change.
	// NOTE: But what if the card was removed and reinserted between monitor ticks?

	//
	// NOTE: copy of selectKeycard
	//

	info, err := kc.SelectApplet()
	if err != nil {
		logger.Error("failed to select applet, card removed?", zap.Error(err))
		return
	}

	appInfo := ToAppInfo(info)
	status.AppInfo = &appInfo

	//logger.Info("card inserted",
	//	zap.String("protocol", protocolToString(status.ActiveProtocol)),
	//	zap.Bool("isKeycard", info.Installed),
	//	zap.Stringer("instanceUID", appInfo.InstanceUID),
	//	zap.Stringer("keyUID", appInfo.KeyUID),
	//	zap.Int("version", appInfo.Version),
	//)

	//
	// NOTE: copy of openSC
	//

	if !info.Initialized {
		status.State = NotKeycard
		return
	}

	pair := kc.pairings.Get(appInfo.InstanceUID.String())

	if pair == nil {
		kc.logger.Debug("pairing not found, pairing now")

		// NOTE: copy of pair
		var pairingInfo *types.PairingInfo
		pairingPassword := DefPairing
		pairingInfo, err = kc.Pair(pairingPassword)
		if err != nil {
			logger.Error("failed to pair", zap.Error(err))
			status.State = PairingError
			return
		}

		pair = pairing.ToPairInfo(pairingInfo)
		err = kc.pairings.Store(appInfo.InstanceUID.String(), pair)
		if err != nil {
			logger.Error("failed to store pairing", zap.Error(err))
			status.State = PairingError
			return
		}
	}

	err = kc.OpenSecureChannel(pair.Index, pair.Key)
	if err != nil {
		logger.Error("failed to open secure channel", zap.Error(err))
		return
	}

	appStatus, err := kc.GetStatusApplication()
	if err != nil {
		logger.Error("failed to get application status", zap.Error(err))
		return
	}

	logger.Info("application status",
		zap.Stringer("instanceUID", appInfo.InstanceUID),
		zap.Any("keyUID", appStatus),
	)
}

func protocolToString(protocol scard.Protocol) string {
	switch protocol {
	case scard.ProtocolT0:
		return "T0"
	case scard.ProtocolT1:
		return "T1"
	default:
		return "unknown"
	}
}
