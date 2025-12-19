package redfish

import (
	"fmt"
	"io"
	"net/http"

	"github.com/stmcginnis/gofish"
	"github.com/stmcginnis/gofish/redfish"
	"go.uber.org/zap"
)

type Client interface {
	Ping() error
	Power(string) error
	GetBasicStatus() (powerState string, bmcStatus string, err error)
	GetInfo() (map[string]string, error)
	GetLog() ([]*redfish.LogEntry, error)
	GetSystemsLogEntries() ([]*redfish.LogEntry, error)
	GetManagersLogEntries() ([]*redfish.LogEntry, error)
	Logout()
}

// Check implantation
var _ Client = (*clientImpl)(nil)

type clientImpl struct {
	client *gofish.APIClient
	logger *zap.SugaredLogger
}

func (c *clientImpl) Ping() error {
	// Use a lightweight request to check connectivity instead of querying Systems collection
	// which can be very large and resource intensive
	resp, err := c.client.Get("/redfish/v1/")
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// Ensure the response body is fully read so the connection can be reused
	_, _ = io.Copy(io.Discard, resp.Body)

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("ping failed with status code: %d", resp.StatusCode)
	}
	return nil
}

// Logout terminates the session with the Redfish service and releases resources
func (c *clientImpl) Logout() {
	if c != nil && c.client != nil {
		c.logger.Debug("Logging out from Redfish service")
		c.client.Logout()
	}
}
