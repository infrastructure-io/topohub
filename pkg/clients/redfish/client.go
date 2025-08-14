package redfish

import (
	"github.com/stmcginnis/gofish"
	"github.com/stmcginnis/gofish/redfish"
	"go.uber.org/zap"
)

type RefishClient interface {
	Ping() error
	Power(string) error
	GetInfo() (map[string]string, error)
	GetLog() ([]*redfish.LogEntry, error)
	GetSystemsLogEntries() ([]*redfish.LogEntry, error)
	GetManagersLogEntries() ([]*redfish.LogEntry, error)
	Logout()
}

// Check implantation
var _ RefishClient = (*redfishClientImpl)(nil)

type redfishClientImpl struct {
	client *gofish.APIClient
	logger *zap.SugaredLogger
}

func (c *redfishClientImpl) Ping() error {
	_, err := c.client.Service.Systems()
	return err
}

// Logout terminates the session with the Redfish service and releases resources
func (c *redfishClientImpl) Logout() {
	if c != nil && c.client != nil {
		c.logger.Debug("Logging out from Redfish service")
		c.client.Logout()
	}
}
