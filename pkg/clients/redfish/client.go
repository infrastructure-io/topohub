package redfish

import (
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
	_, err := c.client.Service.Systems()
	return err
}

// Logout terminates the session with the Redfish service and releases resources
func (c *clientImpl) Logout() {
	if c != nil && c.client != nil {
		c.logger.Debug("Logging out from Redfish service")
		c.client.Logout()
	}
}
