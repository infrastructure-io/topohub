package redfish

import (
	"fmt"

	topohubv1beta1 "github.com/infrastructure-io/topohub/pkg/k8s/apis/topohub.infrastructure.io/v1beta1"
	"github.com/stmcginnis/gofish/redfish"
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
		c.logger.Debugf("system %s, supported reset types: %+v", system.Name, system.SupportedResetTypes)

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
			// https://github.com/stmcginnis/gofish/blob/main/examples/reboot.md
			// Creates a boot override to pxe once
			bootOverride := redfish.Boot{
				// boot from the Pre-Boot EXecution (PXE) environment
				BootSourceOverrideTarget: redfish.PxeBootSourceOverrideTarget,
				// boot (one time) to the Boot Source Override Target
				BootSourceOverrideEnabled: redfish.OnceBootSourceOverrideEnabled,
			}
			c.logger.Infof("pxe reboot %s for System: %+v \n", c.config.Endpoint, system.Name)
			err = system.SetBoot(bootOverride)
			if err != nil {
				return fmt.Errorf("failed to set boot option error:%+v", err)
			}
			err = system.Reset(redfish.ForceRestartResetType)

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
