package redfish

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"time"

	"github.com/stmcginnis/gofish"
	"github.com/stmcginnis/gofish/redfish"
	"go.uber.org/zap"

	redfishstatusData "github.com/infrastructure-io/topohub/pkg/redfishstatus/data"
)

// Client defines the redfish client interface
type RefishClient interface {
	Power(string) error
	GetInfo() (map[string]string, error)
	GetLog() ([]*redfish.LogEntry, error)
	GetSystemsLogEntries() ([]*redfish.LogEntry, error)
	GetManagersLogEntries() ([]*redfish.LogEntry, error)
	Logout()
}

// redfishClient implements the RefishClient interface
type redfishClient struct {
	config gofish.ClientConfig
	logger *zap.SugaredLogger
	client *gofish.APIClient
}

var _ RefishClient = (*redfishClient)(nil)

// NewClient creates a new redfish client
func NewClient(hostCon redfishstatusData.RedfishConnectCon, log *zap.SugaredLogger) (RefishClient, error) {
	url := buildRedfishEndpoint(hostCon)

	// create custom HTTP client
	defaultTransport := http.DefaultTransport.(*http.Transport)
	transport := &http.Transport{
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

	config := gofish.ClientConfig{
		Endpoint:         url,
		Username:         hostCon.Username,
		Password:         hostCon.Password,
		Insecure:         true,
		ReuseConnections: true,
		HTTPClient: &http.Client{
			Transport: transport,
		},
	}

	log.Debugf("create new redfish client for %s", hostCon.IPAddr)
	client, err := gofish.Connect(config)
	if err != nil {
		return nil, fmt.Errorf("failed to connect: %+v", err)
	}
	c := &redfishClient{
		config: config,
		logger: log.Named("redfish").With(
			zap.String("endpoint", url),
		),
		client: client,
	}

	return c, nil
}

// buildRedfishEndpoint builds the redfish endpoint
func buildRedfishEndpoint(redfishCon redfishstatusData.RedfishConnectCon) string {
	protocol := "https"
	if redfishCon.Http {
		protocol = "http"
	}
	return fmt.Sprintf("%s://%s:%d", protocol, redfishCon.IPAddr, redfishCon.Port)
}

// Logout terminates the session with the Redfish service and releases resources
func (c *redfishClient) Logout() {
	if c != nil && c.client != nil {
		c.logger.Debug("Logging out from Redfish service")
		c.client.Logout()
	}
}
