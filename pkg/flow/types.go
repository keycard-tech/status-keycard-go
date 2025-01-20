package flow

type FlowType int
type FlowParams map[string]interface{}
type FlowStatus map[string]interface{}
type RunState int

type restartError struct{}
type giveupError struct{}
type authenticityError struct{}

func restartErr() (e *restartError) {
	return &restartError{}
}

func (e *restartError) Error() string {
	return "restart"
}

func giveupErr() (e *giveupError) {
	return &giveupError{}
}

func (e *giveupError) Error() string {
	return "giveup"
}

func authenticityErr() (e *authenticityError) {
	return &authenticityError{}
}

func (e *authenticityError) Error() string {
	return "authenticity"
}

const (
	GetAppInfo FlowType = iota
	RecoverAccount
	LoadAccount
	Login
	ExportPublic
	Sign
	ChangePIN
	ChangePUK
	ChangePairing
	UnpairThis
	UnpairOthers
	DeleteAccountAndUnpair
	StoreMetadata
	GetMetadata
)

const (
	Idle RunState = iota
	Running
	Paused
	Resuming
	Cancelling
)

const (
	FlowResult    = "keycard.flow-result"
	InsertCard    = "keycard.action.insert-card"
	CardInserted  = "keycard.action.card-inserted"
	SwapCard      = "keycard.action.swap-card"
	EnterPairing  = "keycard.action.enter-pairing"
	EnterPIN      = "keycard.action.enter-pin"
	EnterPUK      = "keycard.action.enter-puk"
	EnterNewPair  = "keycard.action.enter-new-pairing"
	EnterNewPIN   = "keycard.action.enter-new-pin"
	EnterNewPUK   = "keycard.action.enter-new-puk"
	EnterTXHash   = "keycard.action.enter-tx-hash"
	EnterPath     = "keycard.action.enter-bip44-path"
	EnterMnemonic = "keycard.action.enter-mnemonic"
	EnterName     = "keycard.action.enter-cardname"
	EnterWallets  = "keycard.action.enter-wallets"
)

const (
	AppInfo      = "application-info"
	InstanceUID  = "instance-uid"
	FactoryReset = "factory reset"
	KeyUID       = "key-uid"
	FreeSlots    = "free-pairing-slots"
	PINRetries   = "pin-retries"
	PUKRetries   = "puk-retries"
	PairingPass  = "pairing-pass"
	Paired       = "paired"
	NewPairing   = "new-pairing-pass"
	DefPairing   = "KeycardDefaultPairing"
	PIN          = "pin"
	NewPIN       = "new-pin"
	PUK          = "puk"
	NewPUK       = "new-puk"
	MasterKey    = "master-key"
	MasterAddr   = "master-key-address"
	WalleRootKey = "wallet-root-key"
	WalletKey    = "wallet-key"
	EIP1581Key   = "eip1581-key"
	WhisperKey   = "whisper-key"
	EncKey       = "encryption-key"
	ExportedKey  = "exported-key"
	Mnemonic     = "mnemonic"
	MnemonicLen  = "mnemonic-length"
	MnemonicIdxs = "mnemonic-indexes"
	TXHash       = "tx-hash"
	BIP44Path    = "bip44-path"
	TXSignature  = "tx-signature"
	Overwrite    = "overwrite"
	ResolveAddr  = "resolve-addresses"
	ExportMaster = "export-master-address"
	ExportPriv   = "export-private"
	CardMeta     = "card-metadata"
	CardName     = "card-name"
	WalletPaths  = "wallet-paths"
	SkipAuthUID  = "skip-auth-uid"
)

const (
	MaxPINRetries = 3
	MaxPUKRetries = 5
	MaxFreeSlots  = 5
	DefMnemoLen   = 12
	DefPINLen     = 6
	DefPUKLen     = 12
)

const (
	MasterPath      = "m"
	WalletRoothPath = "m/44'/60'/0'/0"
	WalletPath      = WalletRoothPath + "/0"
	Eip1581Path     = "m/43'/60'/1581'"
	WhisperPath     = Eip1581Path + "/0'/0"
	EncryptionPath  = Eip1581Path + "/1'/0"
)
