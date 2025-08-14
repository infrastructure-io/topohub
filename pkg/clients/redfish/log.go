package redfish

import (
	"errors"
	"fmt"

	"github.com/stmcginnis/gofish/redfish"
)

// redfish url: /redfish/v1/Systems/Self/LogServices
func (c *redfishClientImpl) GetLog() ([]*redfish.LogEntry, error) {
	result := []*redfish.LogEntry{}

	// Attached the client to service root
	service := c.client.Service

	// Query the computer systems
	ss, err := service.Systems()
	if err != nil {
		c.logger.Errorf("failed to Query the computer systems: %+v", err)
		return nil, err
	} else if len(ss) == 0 {
		c.logger.Errorf("failed to get system")
		return nil, fmt.Errorf("failed to get system")
	}
	c.logger.Debugf("system amount: %d", len(ss))
	// for n, t := range ss {
	// 	c.logger.Debugf("systems[%d]: %+v", n, *t)
	// }

	// for barel metal case,
	system := ss[0]

	ls, err := system.LogServices()
	if err != nil {
		c.logger.Errorf("failed to Query the log services: %+v", err)
		return nil, err
	} else if len(ls) == 0 {
		c.logger.Errorf("failed to get log service")
		return nil, nil
	}
	c.logger.Debugf("log service amount: %d", len(ls))
	for _, t := range ls {
		if t.Status.State != "Enabled" {
			c.logger.Debugf("log service %s is disabled", t.Name)
			continue
		}

		entries, err := t.Entries()
		if err != nil {
			return nil, err
		} else if len(entries) > 0 {
			c.logger.Debugf("log service entries amount: %d", len(entries))
			result = append(result, entries...)
		}
	}

	// Get manager logs and append them to the result
	managerLogs, err := c.GetManagerLog()
	if err == nil && len(managerLogs) > 0 {
		c.logger.Debugf("adding %d manager log entries", len(managerLogs))
		result = append(result, managerLogs...)
	}

	return result, nil
}

// redfish url: /redfish/v1/Managers/Self/LogServices
func (c *redfishClientImpl) GetManagerLog() ([]*redfish.LogEntry, error) {

	result := []*redfish.LogEntry{}

	// Attached the client to service root
	service := c.client.Service

	// Query the managers
	ms, err := service.Managers()
	if err != nil {
		c.logger.Errorf("failed to Query the managers: %+v", err)
		return nil, err
	} else if len(ms) == 0 {
		c.logger.Errorf("failed to get manager")
		return nil, fmt.Errorf("failed to get manager")
	}
	c.logger.Debugf("manager amount: %d", len(ms))

	// For management controller case
	manager := ms[0]

	ls, err := manager.LogServices()
	if err != nil {
		c.logger.Errorf("failed to Query the manager log services: %+v", err)
		return nil, err
	} else if len(ls) == 0 {
		c.logger.Errorf("failed to get manager log service")
		return nil, nil
	}
	c.logger.Debugf("manager log service amount: %d", len(ls))
	for _, t := range ls {
		if t.Status.State != "Enabled" {
			c.logger.Debugf("manager log service %s is disabled", t.Name)
			continue
		}

		entries, err := t.Entries()
		if err != nil {
			c.logger.Warnf("failed to Query the manager log service entries: %+v", err)
			return nil, err
		} else if len(entries) > 0 {
			c.logger.Debugf("manager log service entries amount: %d", len(entries))
			result = append(result, entries...)
		}
	}

	return result, nil
}

// GetSystemsLogEntries 获取系统日志条目
func (c *redfishClientImpl) GetSystemsLogEntries() ([]*redfish.LogEntry, error) {
	result := []*redfish.LogEntry{}

	// Attached the client to service root
	service := c.client.Service

	// Query the computer systems
	ss, err := service.Systems()
	if err != nil {
		return nil, err
	} else if len(ss) == 0 {
		return nil, errors.New("failed to get system")
	}
	c.logger.Debugf("system amount: %d", len(ss))

	// for barel metal case,
	system := ss[0]

	ls, err := system.LogServices()
	if err != nil {
		c.logger.Errorf("failed to Query the log services: %+v", err)
		return nil, err
	} else if len(ls) == 0 {
		c.logger.Errorf("failed to get log service")
		return nil, nil
	}
	c.logger.Debugf("log service amount: %d", len(ls))
	for _, t := range ls {
		if t.Status.State != "Enabled" {
			c.logger.Debugf("log service %s is disabled", t.Name)
			continue
		}

		entries, err := t.Entries()
		if err != nil {
			c.logger.Warnf("failed to Query the log service entries: %+v", err)
			return nil, err
		} else if len(entries) > 0 {
			c.logger.Debugf("log service entries amount: %d", len(entries))
			result = append(result, entries...)
		}
	}

	return result, nil
}

// GetManagersLogEntries 获取管理器日志条目
func (c *redfishClientImpl) GetManagersLogEntries() ([]*redfish.LogEntry, error) {
	return c.GetManagerLog()
}
