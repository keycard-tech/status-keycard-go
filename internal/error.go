package internal

const (
	ErrorKey         = "error"
	ErrorOK          = "ok"
	ErrorCancel      = "cancel"
	ErrorConnection  = "connection-error"
	ErrorUnknownFlow = "unknown-flow"
	ErrorNotAKeycard = "not-a-keycard"
	ErrorNoKeys      = "no-keys"
	ErrorHasKeys     = "has-keys"
	ErrorRequireInit = "require-init"
	ErrorPairing     = "pairing"
	ErrorUnblocking  = "unblocking"
	ErrorSigning     = "signing"
	ErrorExporting   = "exporting"
	ErrorChanging    = "changing-credentials"
	ErrorLoading     = "loading-keys"
	ErrorStoreMeta   = "storing-metadata"
	ErrorNoData      = "no-data"
	ErrorPCSC        = "no-pcsc"
	ErrorReaderList  = "no-reader-list"
	ErrorNoReader    = "no-reader-found"
)
