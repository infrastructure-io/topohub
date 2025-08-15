package redfish

import (
	"fmt"
	"strings"

	"github.com/stmcginnis/gofish/redfish"

	topohubv1beta1 "github.com/infrastructure-io/topohub/pkg/k8s/apis/topohub.infrastructure.io/v1beta1"
)

// https://github.com/DMTF/Redfish-Tacklebox/blob/main/scripts/rf_power_reset.py
// post request to systems

func (c *redfishClient) Power(bootCmd string) error {
	// Attached the client to service root
	service := c.client.Service
	// Query the computer systems
	ss, err := service.Systems()
	if err != nil {
		c.logger.Errorf("failed to Query the computer systems: %+v", err)
		return err
	}
	if len(ss) == 0 {
		c.logger.Errorf("no system found")
		return fmt.Errorf("no system found")
	}

	for _, system := range ss {
		bootOptions, err := system.BootOptions()
		if err != nil {
			c.logger.Errorf("failed to get boot options: %+v", err)
			return err
		}
		c.logger.Debugf("system %s, boot options: %+v", system.Name, bootOptions)
		c.logger.Debugf("system %s, boot : %+v", system.Name, system.Boot)
		// url: /redfish/v1/Systems/Self/ResetActionInfo
		resetTypes := c.GetSupportedResetTypes(system)
		c.logger.Debugf("system %s, supported reset types: %+v", system.Name, resetTypes)

		switch bootCmd {
		case topohubv1beta1.BootCmdOn:
			fallthrough
		case topohubv1beta1.BootCmdForceOn:
			fallthrough
		case topohubv1beta1.BootCmdForceOff:
			fallthrough
		case topohubv1beta1.BootCmdGracefulShutdown:
			fallthrough
		case topohubv1beta1.BootCmdForceRestart:
			fallthrough
		case topohubv1beta1.BootCmdGracefulRestart:
			c.logger.Infof("operation %s on %s for System: %+v \n", bootCmd, c.config.Endpoint, system.Name)
			err = system.Reset(redfish.ResetType(bootCmd))

		case topohubv1beta1.BootCmdResetPxeOnce:
			// check if the system supports GracefulRestart or ForceRestart
			if !strings.Contains(resetTypes, string(redfish.GracefulRestartResetType)) && !strings.Contains(resetTypes, string(redfish.ForceRestartResetType)) {
				return fmt.Errorf("neither GracefulRestart nor ForceRestart is supported by system %s, supported types: %v", system.Name, resetTypes)
			}

			// https://github.com/stmcginnis/gofish/blob/main/examples/reboot.md
			// Creates a boot override to pxe once
			bootOverride := redfish.Boot{
				// boot from the Pre-Boot EXecution (PXE) environment
				BootSourceOverrideTarget: redfish.PxeBootSourceOverrideTarget,
				// boot (one time) to the Boot Source Override Target
				BootSourceOverrideEnabled: redfish.OnceBootSourceOverrideEnabled,
				BootSourceOverrideMode:    redfish.UEFIBootSourceOverrideMode,
			}
			c.logger.Infof("pxe reboot %s for System: %+v \n", c.config.Endpoint, system.Name)

			err = c.pxeRebootWithRetry(system, bootOverride, resetTypes)
			if err != nil {
				return fmt.Errorf("failed to set boot options: %+v", err)
			}

		default:
			c.logger.Errorf("unknown boot cmd: %+v", bootCmd)
			return fmt.Errorf("unknown boot cmd: %+v", bootCmd)
		}
		if err != nil {
			c.logger.Errorf("failed to operate system %+v: %+v , the host support reset type: %+v\n", system, err, system.SupportedResetTypes)
			return fmt.Errorf("failed to operate ")
		}
	}

	return nil
}

// Lenovo machine Redifish requires an ETag,when the ETag does not match, it may report an error, so add a retry
func (c *redfishClient) pxeRebootWithRetry(system *redfish.ComputerSystem, bootOverride redfish.Boot, resetTypes string) error {
	// Maximum retry attempts
	maxRetries := 3
	var lastErr error

	for i := 0; i < maxRetries; i++ {
		// If this is not the first attempt, refresh the system info to update ETag
		if i > 0 {
			c.logger.Infof("Retry attempt %d for setting PXE boot...", i)
			// Refresh system info to update ETag
			systems, refreshErr := c.client.Service.Systems()
			if refreshErr != nil {
				return fmt.Errorf("failed to refresh system info: %+v", refreshErr)
			}
			if len(systems) == 0 {
				return fmt.Errorf("no systems found during refresh")
			}

			// find the system with the same ID
			originalID := system.ID
			found := false
			for _, s := range systems {
				if s.ID == originalID {
					system = s
					found = true
					break
				}
			}

			if !found {
				return fmt.Errorf("system %s not found after refresh", originalID)
			}
		}

		// set boot options
		if err := system.SetBoot(bootOverride); err != nil {
			c.logger.Errorf("Failed to set boot options: %v, will retry", err)
			lastErr = err
			continue
		}

		// restart system
		var restartType redfish.ResetType
		if strings.Contains(resetTypes, string(redfish.GracefulRestartResetType)) {
			restartType = redfish.GracefulRestartResetType
		} else {
			restartType = redfish.ForceRestartResetType
		}

		c.logger.Infof("using %s restart type for System: %s", restartType, system.Name)
		if err := system.Reset(restartType); err != nil {
			c.logger.Errorf("Reset failed after setting boot options: %v, will retry", err)
			lastErr = err
			continue
		}

		// If we get here, it means SetBoot and Reset both succeeded
		return nil
	}

	return fmt.Errorf("failed to set boot options after %d retries: %+v", maxRetries, lastErr)
}
