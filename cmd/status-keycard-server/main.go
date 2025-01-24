package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"github.com/status-im/status-keycard-go/cmd/status-keycard-server/server"
)

var (
	address    = flag.String("address", "127.0.0.1:0", "host:port to listen")
	rootLogger = zap.NewNop()
)

func init() {
	var err error
	config := zap.NewDevelopmentConfig()
	config.EncoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
	rootLogger, err = config.Build()
	if err != nil {
		fmt.Printf("failed to initialize log: %v\n", err)
	}
	zap.ReplaceGlobals(rootLogger)
}

func main() {
	logger := rootLogger.Named("main")

	flag.Parse()
	go handleInterrupts()

	srv := server.NewServer(rootLogger)
	srv.Setup()

	err := srv.Listen(*address)
	if err != nil {
		logger.Error("failed to start server", zap.Error(err))
		return
	}

	logger.Info("keycard-server started", zap.String("address", srv.Address()))
	srv.Serve()
}

// handleInterrupts catches interrupt signal (SIGTERM/SIGINT) and
// gracefully logouts and stops the node.
func handleInterrupts() {
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(ch)

	<-ch
	os.Exit(0)
}
