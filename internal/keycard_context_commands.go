package internal

import (
	"crypto/sha512"
	"errors"

	"github.com/status-im/keycard-go/types"
	"github.com/status-im/keycard-go/apdu"
	"github.com/ethereum/go-ethereum/crypto"
	"golang.org/x/crypto/pbkdf2"
	"github.com/status-im/keycard-go"
	"github.com/status-im/keycard-go/globalplatform"
	"github.com/status-im/keycard-go/identifiers"
	"golang.org/x/text/unicode/norm"
)
