package internal

import (
	"context"
	"runtime"
	"sync/atomic"
	"time"

	"github.com/ebfe/scard"
	"github.com/ethereum/go-ethereum/crypto"
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
	errKeycardNotConnected   = errors.New("keycard not connected")
	errKeycardNotReady       = errors.New("keycard not ready")
	errNotKeycard            = errors.New("card is not a keycard")
	errKeycardNotInitialized = errors.New("keycard not initialized")
)

type KeycardContextV2 struct {
	KeycardContext

	shutdown     func()
	forceScan    atomic.Bool // Needed to distinguish cardCtx.cancel() from a real shutdown
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
	kc.forceScan.Store(false)

	go kc.cardCommunicationRoutine(ctx)
	kc.monitor(ctx)

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

func (kc *KeycardContextV2) monitor(ctx context.Context) {
	if kc.cardCtx == nil {
		panic("card context is nil")
	}

	logger := kc.logger.Named("monitor")

	go func() {
		defer logger.Debug("monitor stopped")
		// This goroutine will be stopped by cardCtx.Cancel()
		for {
			finish := kc.monitorRoutine(ctx, logger)
			if finish {
				return
			}
		}
	}()
}

func (kc *KeycardContextV2) monitorRoutine(ctx context.Context, logger *zap.Logger) bool {
	// Get current readers list and state
	readers, err := kc.getCurrentReadersState()
	if err != nil {
		logger.Error("failed to get readers state", zap.Error(err))
		kc.status.Reset(InternalError)
		kc.publishStatus()
		return false
	}

	if readers.Empty() && kc.status.State != WaitingForReader {
		kc.status.Reset(WaitingForReader)
		kc.publishStatus()
	}

	kc.scanReadersForKeycard(readers)

	// Wait for readers changes, including new readers
	// https://blog.apdu.fr/posts/2024/08/improved-scardgetstatuschange-for-pnpnotification-special-reader/
	// NOTE: The article states that MacOS is not supported, but works for me on MacOS 15.1.1 (24B91).
	//pnpReader := scard.ReaderState{
	//	Reader:       pnpNotificationReader,
	//	CurrentState: scard.StateUnaware,
	//}
	//rs := append(readers, pnpReader)
	//err = kc.cardCtx.GetStatusChange(rs, infiniteTimeout)

	rs := append(readers)
	err = kc.getStatusChange(ctx, rs, infiniteTimeout)
	if err == scard.ErrCancelled && !kc.forceScan.Load() {
		// Shutdown requested
		return true
	}
	if err != scard.ErrCancelled && err != nil {
		logger.Error("failed to get status change", zap.Error(err))
		return false
	}

	return false
}

func (kc *KeycardContextV2) getStatusChange(ctx context.Context, readersStates ReadersStates, timeout time.Duration) error {
	//return kc.cardCtx.GetStatusChange(readersStates, timeout)

	timer := time.NewTimer(timeout)
	if timeout < 0 {
		timer.Stop() // FIXME: Will it stop, but not tick?
	}

	ticker := time.NewTicker(500 * time.Millisecond)

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-timer.C:
			return scard.ErrTimeout
		case <-ticker.C:
			if len(readersStates) == 0 {
				return nil
			}
			if kc.forceScan.Load() {
				return scard.ErrCancelled
			}

			err := kc.cardCtx.GetStatusChange(readersStates, 100*time.Millisecond)
			if err == scard.ErrTimeout {
				break
			}
			if err != nil {
				return err
			}
			if readersStates.HasChanges() {
				return nil
			}
		}
	}
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

func (kc *KeycardContextV2) scanReadersForKeycard(readers ReadersStates) {
	if !kc.forceScan.Load() &&
		kc.activeReader != "" &&
		readers.Contains(kc.activeReader) &&
		readers.ReaderHasCard(kc.activeReader) {
		// active reader is not selected yet or is still present, no need to connect a card
		return
	}

	if readers.Empty() {
		return
	}

	kc.forceScan.Store(false)
	kc.resetCardConnection(false)

	readerWithCardIndex, ok := readers.ReaderWithCardIndex()
	if !ok {
		kc.logger.Debug("no card found on any readers")
		kc.status.Reset(WaitingForCard)
		kc.publishStatus()
		return
	}

	kc.logger.Debug("card found", zap.Int("index", readerWithCardIndex))

	err := kc.connectKeycard(readers[readerWithCardIndex].Reader)
	if err != nil && !errors.Is(err, errNotKeycard) && !errors.Is(err, errKeycardNotInitialized) {
		kc.logger.Error("failed to connect keycard", zap.Error(err))
	}

	kc.publishStatus()
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

	appInfo, err := kc.selectApplet()
	if err != nil {
		kc.status.State = ConnectionError
		return errors.Wrap(err, "failed to select applet")
	}

	//
	// NOTE: copy of openSC
	//

	kc.status.AppInfo = appInfo

	if !appInfo.Installed {
		kc.status.State = NotKeycard
		return errNotKeycard
	}

	if !appInfo.Initialized {
		kc.status.State = EmptyKeycard
		return errKeycardNotInitialized
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

		// After successful pairing, we should `SelectApplet` again to update the ApplicationInfo
		appInfo, err = kc.selectApplet()
		if err != nil {
			kc.status.State = ConnectionError
			return errors.Wrap(err, "failed to select applet")
		}
		kc.status.AppInfo = appInfo
	}

	err = kc.OpenSecureChannel(pair.Index, pair.Key)
	if err != nil {
		kc.status.State = ConnectionError
		return errors.Wrap(err, "failed to open secure channel")
	}

	err = kc.updateApplicationStatus()
	if err != nil {
		return errors.Wrap(err, "failed to get application status")
	}

	err = kc.updateMetadata()
	if err != nil {
		return errors.Wrap(err, "failed to get metadata")
	}

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
		kc.forceScan.Store(true)
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
	kc.forceScan.Store(false)
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

func (kc *KeycardContextV2) keycardReady() bool {
	return kc.keycardConnected() && kc.status.State == Ready
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

func (kc *KeycardContextV2) selectApplet() (*ApplicationInfoV2, error) {
	info, err := kc.SelectApplet()
	if err != nil {
		kc.status.State = ConnectionError
		return nil, err
	}

	return ToAppInfoV2(info), err
}

func (kc *KeycardContextV2) updateApplicationStatus() error {
	appStatus, err := kc.cmdSet.GetStatusApplication()
	kc.status.AppStatus = appStatus

	if err != nil {
		kc.status.State = ConnectionError
		return err
	}

	kc.status.State = Ready

	if appStatus != nil {
		if appStatus.PinRetryCount == 0 {
			kc.status.State = BlockedPIN
		}
		if appStatus.PUKRetryCount == 0 {
			kc.status.State = BlockedPUK
		}
	}

	return nil
}

func (kc *KeycardContextV2) updateMetadata() error {
	metadata, err := kc.GetMetadata()
	if err != nil {
		return err
	}

	kc.status.Metadata = metadata
	return nil
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

	defer func() {
		// Update app status to get the new pin remaining attempts
		// Although we can parse the `err` as `keycard.WrongPINError`, it won't work for `err == nil`.
		err := kc.updateApplicationStatus()
		if err != nil {
			kc.logger.Error("failed to update app status after verifying pin")
		}
		kc.publishStatus()
	}()

	err := kc.cmdSet.VerifyPIN(pin)
	return kc.checkSCardError(err, "VerifyPIN")
}

func (kc *KeycardContextV2) ChangePIN(pin string) error {
	if !kc.keycardConnected() {
		return errKeycardNotConnected
	}

	defer func() {
		err := kc.updateApplicationStatus()
		if err != nil {
			kc.logger.Error("failed to update app status after changing pin")
		}
		kc.publishStatus()
	}()

	err := kc.cmdSet.ChangePIN(pin)
	return kc.checkSCardError(err, "ChangePIN")
}

func (kc *KeycardContextV2) UnblockPIN(puk string, newPIN string) error {
	if !kc.keycardConnected() {
		return errKeycardNotConnected
	}

	defer func() {
		err := kc.updateApplicationStatus()
		if err != nil {
			kc.logger.Error("failed to update app status after unblocking")
		}
		kc.publishStatus()
	}()

	err := kc.cmdSet.UnblockPIN(puk, newPIN)
	return kc.checkSCardError(err, "UnblockPIN")
}

func (kc *KeycardContextV2) ChangePUK(puk string) error {
	if !kc.keycardReady() {
		return errKeycardNotReady
	}

	defer func() {
		err := kc.updateApplicationStatus()
		if err != nil {
			kc.logger.Error("failed to update app status after changing pin")
		}
		kc.publishStatus()
	}()

	err := kc.cmdSet.ChangePUK(puk)
	return kc.checkSCardError(err, "ChangePUK")
}

func (kc *KeycardContextV2) GenerateMnemonic(mnemonicLength int) ([]int, error) {
	if !kc.keycardReady() {
		return nil, errKeycardNotReady
	}

	indexes, err := kc.cmdSet.GenerateMnemonic(mnemonicLength / 3)
	return indexes, kc.checkSCardError(err, "GenerateMnemonic")
}

func (kc *KeycardContextV2) LoadMnemonic(mnemonic string, password string) ([]byte, error) {
	if !kc.keycardReady() {
		return nil, errKeycardNotReady
	}

	var keyUID []byte
	var err error

	defer func() {
		if err != nil {
			return
		}
		kc.status.AppInfo.KeyUID = keyUID
		kc.publishStatus()
	}()

	seed := kc.mnemonicToBinarySeed(mnemonic, password)
	keyUID, err = kc.loadSeed(seed)
	return keyUID, kc.checkSCardError(err, "LoadMnemonic")
}

func (kc *KeycardContextV2) FactoryReset() error {
	if !kc.keycardConnected() {
		return errKeycardNotConnected
	}

	kc.status.Reset(FactoryResetting)
	kc.publishStatus()

	kc.logger.Debug("factory reset")
	err := kc.KeycardContext.FactoryReset(true)

	// Reset card connection to read the card data
	kc.resetCardConnection(true)
	return err
}

func (kc *KeycardContextV2) GetMetadata() (*Metadata, error) {
	if !kc.keycardConnected() {
		return nil, errKeycardNotConnected
	}

	data, err := kc.cmdSet.GetData(keycard.P1StoreDataPublic)
	if err != nil {
		return nil, kc.checkSCardError(err, "GetMetadata")
	}

	if len(data) == 0 {
		return nil, nil
	}

	metadata, err := types.ParseMetadata(data)
	if err != nil {
		return nil, errors.Wrap(err, "failed to parse metadata")
	}

	return ToMetadata(metadata), nil
}

func (kc *KeycardContextV2) exportedKeyToAddress(key *types.ExportedKey) (string, error) {
	if key.PubKey() == nil {
		return "", nil
	}

	ecdsaPubKey, err := crypto.UnmarshalPubkey(key.PubKey())
	if err != nil {
		return "", errors.Wrap(err, "failed to unmarshal public key")
	}

	return crypto.PubkeyToAddress(*ecdsaPubKey).Hex(), nil
}

func (kc *KeycardContextV2) exportKey(path string, exportOption uint8) (*KeyPair, error) {
	// 1. As for today, it's pointless to use the 'current path' feature. So we always derive.
	// 2. We keep this workaround for `makeCurrent` to mitigate a bug in an older version of the Keycard applet
	//    that doesn't correctly export the public key for the master path unless it is also the current path.
	const derive = true
	makeCurrent := path == MasterPath

	exportedKey, err := kc.cmdSet.ExportKeyExtended(derive, makeCurrent, exportOption, path)
	if err != nil {
		return nil, kc.checkSCardError(err, "ExportKeyExtended")
	}

	address, err := kc.exportedKeyToAddress(exportedKey)
	if err != nil {
		return nil, errors.Wrap(err, "failed to convert key to address")
	}

	return &KeyPair{
		Address:    address,
		PublicKey:  exportedKey.PubKey(),
		PrivateKey: exportedKey.PrivKey(),
		ChainCode:  exportedKey.ChainCode(),
	}, nil
}

func (kc *KeycardContextV2) ExportLoginKeys() (*LoginKeys, error) {
	if !kc.keycardReady() {
		return nil, errKeycardNotReady
	}

	var err error
	keys := &LoginKeys{}

	keys.EncryptionPrivateKey, err = kc.exportKey(EncryptionPath, keycard.P2ExportKeyPrivateAndPublic)
	if err != nil {
		return nil, err
	}

	keys.WhisperPrivateKey, err = kc.exportKey(WhisperPath, keycard.P2ExportKeyPrivateAndPublic)
	if err != nil {
		return nil, err
	}

	return keys, err
}

func (kc *KeycardContextV2) ExportRecoverKeys() (*RecoverKeys, error) {
	if !kc.keycardReady() {
		return nil, errKeycardNotReady
	}

	loginKeys, err := kc.ExportLoginKeys()
	if err != nil {
		return nil, err
	}

	keys := &RecoverKeys{
		LoginKeys: *loginKeys,
	}

	keys.EIP1581key, err = kc.exportKey(Eip1581Path, keycard.P2ExportKeyPublicOnly)
	if err != nil {
		return nil, err
	}

	rootExportOptions := map[bool]uint8{
		true:  keycard.P2ExportKeyExtendedPublic,
		false: keycard.P2ExportKeyPublicOnly,
	}
	keys.WalletRootKey, err = kc.exportKey(WalletRoothPath, rootExportOptions[kc.status.KeycardSupportsExtendedKeys()])
	if err != nil {
		return nil, err
	}

	keys.WalletKey, err = kc.exportKey(WalletPath, keycard.P2ExportKeyPublicOnly)
	if err != nil {
		return nil, err
	}

	keys.MasterKey, err = kc.exportKey(MasterPath, keycard.P2ExportKeyPublicOnly)
	if err != nil {
		return nil, err
	}

	return keys, err
}
