package httpserver

import (
	"context"
	"fmt"
	"net/http"
	"path"
	"sync"

	"github.com/infrastructure-io/topohub/pkg/config"
	"github.com/infrastructure-io/topohub/pkg/log"
	"go.uber.org/zap"
)

type HttpManager interface {
	Run()
	Stop()
}

type httpServer struct {
	config *config.AgentConfig
	log    *zap.SugaredLogger

	server        *http.Server
	stopOnce      sync.Once
	stopCtx       context.Context
	stopCtxCancel context.CancelFunc
}

func NewHttpServer(config config.AgentConfig) HttpManager {
	ctx, cancel := context.WithCancel(context.Background())

	server := &httpServer{
		config:        &config,
		stopCtx:       ctx,
		stopCtxCancel: cancel,
		log:           log.Logger.Named("httpserver"),
	}

	// Create file server handler
	fileServer := http.FileServer(http.Dir(config.StoragePathHttp))

	// Create mux and register routes
	mux := http.NewServeMux()
	mux.Handle("/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Clean the request path to prevent directory traversal
		cleanPath := path.Clean(r.URL.Path)
		r.URL.Path = cleanPath
		fileServer.ServeHTTP(w, r)
	}))

	server.server = &http.Server{
		Addr:    fmt.Sprintf(":%s", config.HttpPort),
		Handler: mux,
	}

	return server
}

func (s *httpServer) Run() {
	go func() {
		s.log.Infof("Starting HTTP server on address %s , root path: %s", s.server.Addr, s.config.StoragePathHttp)
		if err := s.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			s.log.Panicf("HTTP server error: %v", err)
		}
	}()
}

func (s *httpServer) Stop() {
	s.stopOnce.Do(func() {
		s.stopCtxCancel()
		if err := s.server.Shutdown(context.Background()); err != nil {
			s.log.Panicf("Error shutting down HTTP server: %v", err)
		}
		s.log.Info("HTTP server stopped")
	})
}
