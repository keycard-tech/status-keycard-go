package main

import (
	"flag"
	"os"
	"os/signal"
	"syscall"

	"github.com/status-im/status-keycard-go/cmd/status-keycard-server/server"
	"go.uber.org/zap"
	"fmt"
)

var (
	address    = flag.String("address", "127.0.0.1:0", "host:port to listen")
	rootLogger = zap.NewNop()
)

func init() {
	var err error
	rootLogger, err = zap.NewDevelopment()
	if err != nil {
		fmt.Printf("failed to initialize log: %v\n", err)
	}
	zap.ReplaceGlobals(rootLogger)
}

func main() {
	logger := rootLogger.Named("main")

	flag.Parse()
	go handleInterrupts()

	// TEMP:
	//response := api.Start("")
	//fmt.Println(response)

	srv := server.NewServer(rootLogger)
	srv.Setup()

	err := srv.Listen(*address)
	if err != nil {
		logger.Error("failed to start server", zap.Error(err))
		return
	}

	logger.Info("keycard-server started", zap.String("address", srv.Address()))
	srv.RegisterMobileAPI()
	srv.Serve()
}

// handleInterrupts catches interrupt signal (SIGTERM/SIGINT) and
// gracefully logouts and stops the node.
func handleInterrupts() {
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(ch)

	_ = <-ch
	os.Exit(0)
}
