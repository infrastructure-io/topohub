package debug

import (
	"net"
	"net/http"
	"net/http/pprof"
	"time"

	"github.com/infrastructure-io/topohub/pkg/log"
)

const (
	// ReadHeaderTimeout is the amount of time allowed to read
	// request headers.
	// HTTP timeouts are necessary to expire inactive connections
	// and failing to do so might make the application vulnerable
	// to attacks like slowloris which work by sending data very slow,
	// which in case of no timeout will keep the connection active
	// eventually leading to a denial-of-service (DoS) attack.
	// References:
	// - https://en.wikipedia.org/wiki/Slowloris_(computer_security)
	ReadHeaderTimeout = 32 * time.Second
)

func installHandlerForPProf(mux *http.ServeMux) {
	mux.HandleFunc("/debug/pprof/", pprof.Index)
	mux.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
	mux.HandleFunc("/debug/pprof/profile", pprof.Profile)
	mux.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
	mux.HandleFunc("/debug/pprof/trace", pprof.Trace)
}

// RunPProf start a http server to enable pprof.
func RunPProf(pprofAddress string, pprofPort string) {
	if pprofAddress != "" && pprofPort != "" {
		mux := http.NewServeMux()
		installHandlerForPProf(mux)
		addr := net.JoinHostPort(pprofAddress, pprofPort)
		log.Logger.Infof("Starting pprof on address %s", addr)
		go func() {
			httpServer := http.Server{
				Addr:              addr,
				Handler:           mux,
				ReadHeaderTimeout: ReadHeaderTimeout,
			}
			if err := httpServer.ListenAndServe(); err != nil {
				log.Logger.Errorf("Failed to start profiling server: %v", err)
			}
		}()
	}
}
