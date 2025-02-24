package internal

import (
	"fmt"

	"go.uber.org/zap"

	"github.com/status-im/status-keycard-go/internal/logging"
	"github.com/status-im/status-keycard-go/pkg/pairing"
)

type Option func(*KeycardContextV2)

func WithStorage(store *pairing.Store) Option {
	return func(k *KeycardContextV2) {
		k.pairings = store
	}
}

func WithLogging(enabled bool, filePath string) Option {
	return func(k *KeycardContextV2) {
		var logger *zap.Logger

		defer func() {
			zap.ReplaceGlobals(logger)
			k.logger = zap.L().Named("keycard")
		}()

		if !enabled {
			logger = zap.NewNop()
			return
		}

		var err error
		logger, err = buildLogger(filePath)

		if err != nil {
			fmt.Printf("failed to initialize log: %v\n", err)
		}
	}
}

func buildLogger(outputFilePath string) (*zap.Logger, error) {
	if outputFilePath != "" {
		// Use production format and output to file
		return logging.BuildProductionLogger(outputFilePath)
	}

	// Use development format and output to console
	return logging.BuildDevelopmentLogger()
}
