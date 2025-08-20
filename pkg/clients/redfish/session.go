package redfish

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"reflect"
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
var _ pool.ClientOperations[RedfishClient] = (*RedfishClientOperations)(nil)

type RedfishClientOperations struct {
	cfg *RedfishSessionConfig
	log *zap.SugaredLogger
}

func (o *RedfishClientOperations) NewClient(cfg any) (RedfishClient, error) {
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

func (o *RedfishClientOperations) Ping(client RedfishClient) error {
	return client.Ping()
}

func (RedfishClientOperations) Compare(old, new any) bool {
	return reflect.DeepEqual(old, new)
}

func (o *RedfishClientOperations) Refresh(oldClient RedfishClient, cfg any) (RedfishClient, error) {
	newcli, err := o.NewClient(cfg)
	if err != nil {
		return nil, err
	}
	oldClient.Logout()
	return newcli, nil
}

func (RedfishClientOperations) Close(client RedfishClient) error {
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
func newRedfishClient(cfg *RedfishSessionConfig, logger *zap.SugaredLogger) (RedfishClient, error) {
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
			Transport: customTransport(),
		},
	}
	logger = logger.With(zap.String("endpoint", config.Endpoint))
	logger.Debugf("create new redfish client for %s", config.Endpoint)
	client, err := gofish.Connect(config)
	if err != nil {
		return nil, fmt.Errorf("connect to redfish server failed, err: %+v", err)
	}
	return &redfishClientImpl{
		logger: logger,
		client: client,
	}, nil
}

// customTransport returns a custom http transport
func customTransport() *http.Transport {
	defaultTransport := http.DefaultTransport.(*http.Transport)
	return &http.Transport{
		Proxy: defaultTransport.Proxy,
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			return net.DialTimeout(network, addr, 5*time.Second)
		},
		MaxIdleConns:          defaultTransport.MaxIdleConns,
		MaxIdleConnsPerHost:   defaultTransport.MaxIdleConnsPerHost, // max idle connections per host
		IdleConnTimeout:       defaultTransport.IdleConnTimeout,     // idle connection timeout
		ExpectContinueTimeout: defaultTransport.ExpectContinueTimeout,
		TLSHandshakeTimeout:   defaultTransport.TLSHandshakeTimeout,
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: true,
		},
	}
}
