package redfish

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"reflect"
	"sync"
	"time"

	"github.com/stmcginnis/gofish"
	"go.uber.org/zap"

	"github.com/infrastructure-io/topohub/pkg/clients/pool"
	"github.com/infrastructure-io/topohub/pkg/log"
)

type RedfishSessionConfig struct {
	// IPAddr is the IP address of the redfish server, required.
	IPAddr string
	// Port is the port of the redfish server. Default is 443 if https is true, otherwise 80.
	Port int
	// Https is true if the redfish server uses https. Default is false.
	Https bool
	// Username is the username of the redfish server, required.
	Username string
	// Password is the password of the redfish server, required.
	Password string
}

func (c *RedfishSessionConfig) VerifyAndSetDefault() error {
	if c.IPAddr == "" {
		return fmt.Errorf("ip addr must not be empty")
	}
	if c.Port == 0 {
		if c.Https {
			c.Port = 443
		} else {
			c.Port = 80
		}
	}
	if c.Username == "" || c.Password == "" {
		return fmt.Errorf("username and password must not be empty")
	}
	return nil
}

func (c *RedfishSessionConfig) URL() string {
	var scheme string
	if c.Https {
		scheme = "https"
	} else {
		scheme = "http"
	}
	return fmt.Sprintf("%s://%s:%d", scheme, c.IPAddr, c.Port)
}

func (c *RedfishSessionConfig) SessionID() string {
	return fmt.Sprintf("%s@%s:%d", c.Username, c.IPAddr, c.Port)
}

// NewRedfishClientOperations creates a new redfish client operations.
// Example for create a redfish client session pool.
//
//	pool := pool.NewSessionPool(ctx, NewRedfishClientOperations(nil))
func NewRedfishClientOperations(logger *zap.SugaredLogger) *RedfishClientOperations {
	if logger == nil {
		logger = log.Logger.Named("redfishClientOperations")
	}
	return &RedfishClientOperations{
		log: logger,
	}
}

// Check implantation
var _ pool.ClientOperations[Client] = (*RedfishClientOperations)(nil)

type RedfishClientOperations struct {
	cfg *RedfishSessionConfig
	log *zap.SugaredLogger
}

func (o *RedfishClientOperations) NewClient(cfg any) (Client, error) {
	redfishCfg, err := verifyRedfishSessionConfig(cfg)
	if err != nil {
		return nil, err
	}
	cli, err := newRedfishClient(redfishCfg, nil)
	if err != nil {
		return nil, fmt.Errorf("new redfish client failed: %w", err)
	}
	o.cfg = redfishCfg
	return cli, nil
}

func (o *RedfishClientOperations) Ping(client Client) error {
	return client.Ping()
}

func (RedfishClientOperations) Compare(old, new any) bool {
	return reflect.DeepEqual(old, new)
}

func (o *RedfishClientOperations) Refresh(oldClient Client, cfg any) (Client, error) {
	newcli, err := o.NewClient(cfg)
	if err != nil {
		return nil, err
	}
	oldClient.Logout()
	return newcli, nil
}

func (RedfishClientOperations) Close(client Client) error {
	client.Logout()
	return nil
}

func verifyRedfishSessionConfig(cfg any) (*RedfishSessionConfig, error) {
	if cfg == nil {
		return nil, fmt.Errorf("redfish session config must not be nil")
	}
	redfishCfg, ok := cfg.(*RedfishSessionConfig)
	if !ok {
		return nil, fmt.Errorf("invalid redfish session config")
	}
	if err := redfishCfg.VerifyAndSetDefault(); err != nil {
		return nil, err
	}
	return redfishCfg, nil
}

// newRedfishClient creates a new redfish client
func newRedfishClient(cfg *RedfishSessionConfig, logger *zap.SugaredLogger) (Client, error) {
	if logger == nil {
		logger = log.Logger.Named("redfishClient")
	}
	url := url.URL{
		Host: fmt.Sprintf("%s:%d", cfg.IPAddr, cfg.Port),
	}
	if cfg.Https {
		url.Scheme = "https"
	} else {
		url.Scheme = "http"
	}
	config := gofish.ClientConfig{
		Endpoint:         url.String(),
		Username:         cfg.Username,
		Password:         cfg.Password,
		Insecure:         true,
		ReuseConnections: true,
		HTTPClient: &http.Client{
			Transport: getSharedTransport(),
		},
	}
	logger = logger.With(zap.String("endpoint", config.Endpoint))
	logger.Debugf("create new redfish client for %s", config.Endpoint)
	client, err := gofish.Connect(config)
	if err != nil {
		return nil, fmt.Errorf("connect to redfish server failed, err: %+v", err)
	}
	return &clientImpl{
		logger: logger,
		client: client,
	}, nil
}

var (
	sharedTransport     *http.Transport
	sharedTransportOnce sync.Once
)

// getSharedTransport returns a shared http transport
func getSharedTransport() *http.Transport {
	sharedTransportOnce.Do(func() {
		defaultTransport := http.DefaultTransport.(*http.Transport)
		sharedTransport = &http.Transport{
			Proxy: defaultTransport.Proxy,
			DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
				return net.DialTimeout(network, addr, 5*time.Second)
			},
			MaxIdleConns:          10000, // Increase global pool size to handle many hosts
			MaxIdleConnsPerHost:   5,     // Slight increase for concurrency per host
			IdleConnTimeout:       90 * time.Second,
			ExpectContinueTimeout: defaultTransport.ExpectContinueTimeout,
			TLSHandshakeTimeout:   defaultTransport.TLSHandshakeTimeout,
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true,
				MinVersion:         tls.VersionTLS10,
				// Configure TLS to handle servers with weak DH keys
				CipherSuites: []uint16{
					// Specify cipher suites that don't use DH key exchange
					tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
					tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
					tls.TLS_ECDHE_RSA_WITH_AES_128_CBC_SHA,
					tls.TLS_ECDHE_RSA_WITH_AES_256_CBC_SHA,
					tls.TLS_RSA_WITH_AES_128_GCM_SHA256,
					tls.TLS_RSA_WITH_AES_256_GCM_SHA384,
					tls.TLS_RSA_WITH_AES_128_CBC_SHA,
					tls.TLS_RSA_WITH_AES_256_CBC_SHA,
				},
			},
		}
	})
	return sharedTransport
}
