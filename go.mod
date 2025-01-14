module github.com/status-im/status-keycard-go

go 1.21.0

toolchain go1.21.8

require (
	github.com/ebfe/scard v0.0.0-20190212122703-c3d1b1916a95
	github.com/ethereum/go-ethereum v1.10.26
	github.com/gorilla/rpc v1.2.1
	github.com/gorilla/websocket v1.4.2
	github.com/pkg/errors v0.9.1
	github.com/status-im/keycard-go v0.3.3
	go.uber.org/zap v1.9.1
	golang.org/x/crypto v0.32.0
	golang.org/x/text v0.21.0
)

require (
	github.com/btcsuite/btcd/btcec/v2 v2.2.0 // indirect
	github.com/decred/dcrd/dcrec/secp256k1/v4 v4.0.1 // indirect
	github.com/go-stack/stack v1.8.1 // indirect
	go.uber.org/atomic v1.11.0 // indirect
	go.uber.org/multierr v1.11.0 // indirect
	golang.org/x/sys v0.29.0 // indirect
)

replace github.com/ebfe/scard => github.com/keycard-tech/scard v0.0.0-20241212105412-f6a0ad2a2912
