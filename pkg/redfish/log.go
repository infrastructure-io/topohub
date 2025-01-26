package redfish

import (
	"fmt"

	"github.com/stmcginnis/gofish/redfish"
)

func (c *redfishClient) GetLog() ([]*redfish.LogEntry, error) {

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
			c.logger.Errorf("failed to Query the log service entries: %+v", err)
			return nil, err
		} else if len(entries) > 0 {
			c.logger.Debugf("log service entries amount: %d", len(entries))
			result = append(result, entries...)
		}
	}

	return result, nil

}
