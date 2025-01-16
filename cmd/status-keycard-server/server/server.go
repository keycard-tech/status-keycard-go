package server

import (
	"context"
	"net"
	"net/http"
	"strconv"
	"sync"
	"time"
	"github.com/gorilla/websocket"

	"github.com/pkg/errors"

	"github.com/status-im/status-keycard-go/pkg/session"
	"go.uber.org/zap"
	"github.com/status-im/status-keycard-go/signal"
	"os"
)

type Server struct {
	logger          *zap.Logger
	server          *http.Server
	listener        net.Listener
	mux             *http.ServeMux
	connectionsLock sync.Mutex
	connections     map[*websocket.Conn]struct{}
	address         string
}

func NewServer(logger *zap.Logger) *Server {
	return &Server{
		logger:      logger.Named("server"),
		connections: make(map[*websocket.Conn]struct{}, 1),
	}
}

func (s *Server) Address() string {
	return s.address
}

func (s *Server) Port() (int, error) {
	_, portString, err := net.SplitHostPort(s.address)
	if err != nil {
		return 0, err
	}
	return strconv.Atoi(portString)
}

func (s *Server) Setup() {
	signal.SetKeycardSignalHandler(s.signalHandler)
}

func (s *Server) signalHandler(data []byte) {
	s.connectionsLock.Lock()
	defer s.connectionsLock.Unlock()

	deleteConnection := func(connection *websocket.Conn) {
		delete(s.connections, connection)
		err := connection.Close()
		if err != nil {
			s.logger.Error("failed to close connection", zap.Error(err))
		}
	}

	for connection := range s.connections {
		err := connection.SetWriteDeadline(time.Now().Add(5 * time.Second))
		if err != nil {
			s.logger.Error("failed to set write deadline", zap.Error(err))
			deleteConnection(connection)
			continue
		}

		err = connection.WriteMessage(websocket.TextMessage, data)
		if err != nil {
			s.logger.Error("failed to write signal message", zap.Error(err))
			deleteConnection(connection)
		}
	}
}

func (s *Server) Listen(address string) error {
	if s.server != nil {
		return errors.New("server already started")
	}

	_, _, err := net.SplitHostPort(address)
	if err != nil {
		return errors.Wrap(err, "invalid address")
	}

	s.server = &http.Server{
		Addr:              address,
		ReadHeaderTimeout: 5 * time.Second,
	}

	rpcServer, err := session.CreateRPCServer()
	if err != nil {
		s.logger.Error("failed to create PRC server", zap.Error(err))
		os.Exit(1)
	}

	s.mux = http.NewServeMux()
	s.mux.HandleFunc("/signals", s.signals)
	s.mux.Handle("/rpc", rpcServer)
	s.server.Handler = s.mux

	s.listener, err = net.Listen("tcp", address)
	if err != nil {
		return err
	}

	s.address = s.listener.Addr().String()

	return nil
}

func (s *Server) Serve() {
	err := s.server.Serve(s.listener)
	if !errors.Is(err, http.ErrServerClosed) {
		s.logger.Error("signals server closed with error", zap.Error(err))
	}
}

func (s *Server) Stop(ctx context.Context) {
	for connection := range s.connections {
		err := connection.Close()
		if err != nil {
			s.logger.Error("failed to close connection", zap.Error(err))
		}
		delete(s.connections, connection)
	}

	err := s.server.Shutdown(ctx)
	if err != nil {
		s.logger.Error("failed to shutdown signals server", zap.Error(err))
	}

	s.server = nil
	s.address = ""
}

func (s *Server) signals(w http.ResponseWriter, r *http.Request) {
	s.connectionsLock.Lock()
	defer s.connectionsLock.Unlock()

	upgrader := websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool {
			return true // Accepting all requests
		},
	}

	connection, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		s.logger.Error("failed to upgrade connection", zap.Error(err))
		return
	}
	s.logger.Debug("new websocket connection")

	s.connections[connection] = struct{}{}
}
