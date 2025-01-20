package internal

const (
	MaxPINRetries = 3
	MaxPUKRetries = 5
	MaxFreeSlots  = 5
	DefMnemoLen   = 12
	DefPINLen     = 6
	DefPUKLen     = 12
	DefPairing    = "KeycardDefaultPairing"
)

const (
	MasterPath      = "m"
	WalletRoothPath = "m/44'/60'/0'/0"
	WalletPath      = WalletRoothPath + "/0"
	Eip1581Path     = "m/43'/60'/1581'"
	WhisperPath     = Eip1581Path + "/0'/0"
	EncryptionPath  = Eip1581Path + "/1'/0"
)
