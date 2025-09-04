package hostoperation

import (
	"fmt"
	"strings"

	"go.uber.org/zap"

	"github.com/infrastructure-io/topohub/pkg/clients/redfish"
	topohubv1beta1 "github.com/infrastructure-io/topohub/pkg/k8s/apis/topohub.infrastructure.io/v1beta1"
)

// performRedfishOperation executes a power operation via Redfish
func (r *HostOperationController) performRedfishOperation(
	hostEndpoint *topohubv1beta1.HostEndpoint,
	action string,
	username string,
	password string,
	hostOp *topohubv1beta1.HostOperation,
) error {
	logger := r.log.With(
		zap.String("hostEndpoint", hostEndpoint.Name),
		zap.String("action", action),
	)

	// Create Redfish session configuration
	sessionCfg := &redfish.RedfishSessionConfig{
		Username: username,
		Password: password,
		IPAddr:   hostEndpoint.Spec.IPAddr,
		Port:     int(*hostEndpoint.Spec.Port),
		Https:    *hostEndpoint.Spec.HTTPS,
	}

	// Get or create a Redfish session
	session, err := r.redfishPool.GetOrCreate(sessionCfg.SessionID(), sessionCfg)
	if err != nil {
		logger.Errorf("Failed to create Redfish session: %v", err)
		hostOp.Status.Status = topohubv1beta1.HostOperationStatusFailed
		hostOp.Status.Message = fmt.Sprintf("Failed to create Redfish session: %v", err)
		return err
	}

	// Get the Redfish client
	c := session.GetClient()

	// Check if this is a standard power operation
	isStandardPowerOp := false
	switch action {
	case topohubv1beta1.RedfishCmdOn,
		topohubv1beta1.RedfishCmdForceOn,
		topohubv1beta1.RedfishCmdForceOff,
		topohubv1beta1.RedfishCmdGracefulShutdown,
		topohubv1beta1.RedfishCmdForceRestart,
		topohubv1beta1.RedfishCmdGracefulRestart:
		isStandardPowerOp = true
	}

	// Execute the appropriate operation based on action type
	if isStandardPowerOp {
		// Standard power operation
		err = c.Power(action)
		if err != nil {
			logger.Errorf("Failed to execute power operation %s: %v", action, err)
			hostOp.Status.Status = topohubv1beta1.HostOperationStatusFailed
			hostOp.Status.Message = fmt.Sprintf("Failed to execute power operation %s: %v", action, err)
			return err
		}
		hostOp.Status.Status = topohubv1beta1.HostOperationStatusSuccess
		hostOp.Status.Message = fmt.Sprintf("Successfully performed %s via Redfish for %s", action, hostEndpoint.Spec.IPAddr)
	} else if action == topohubv1beta1.RedfishCmdPxeReboot {
		// PXE boot operation
		if strings.ToLower(hostEndpoint.Spec.PxeBootType) == topohubv1beta1.PxeBootTypeIPMI {
			// Use IPMI command to set PXE boot
			logger.Infof("Using IPMI to set PXE boot for %s", hostEndpoint.Spec.IPAddr)
			err = r.performIPMIPXEBoot(sessionCfg.IPAddr, sessionCfg.Username, sessionCfg.Password)
			if err != nil {
				logger.Errorf("Failed to perform PXE boot via IPMI: %v", err)
				hostOp.Status.Status = topohubv1beta1.HostOperationStatusFailed
				hostOp.Status.Message = fmt.Sprintf("Failed to perform PXE boot via IPMI: %v", err)
				return err
			}
			hostOp.Status.Status = topohubv1beta1.HostOperationStatusSuccess
			hostOp.Status.Message = fmt.Sprintf("Successfully performed PXE boot via IPMI for %s", hostEndpoint.Spec.IPAddr)
		} else {
			// Use default Redfish client
			logger.Infof("Using Redfish to set PXE boot for %s", hostEndpoint.Spec.IPAddr)
			err = c.Power(action)
			if err != nil {
				logger.Errorf("Failed to execute PXE boot via Redfish: %v", err)
				hostOp.Status.Status = topohubv1beta1.HostOperationStatusFailed
				hostOp.Status.Message = fmt.Sprintf("Failed to execute PXE boot via Redfish: %v", err)
				return err
			}
			hostOp.Status.Status = topohubv1beta1.HostOperationStatusSuccess
			hostOp.Status.Message = fmt.Sprintf("Successfully performed PXE boot via Redfish method for %s", hostEndpoint.Spec.IPAddr)
			logger.Info(hostOp.Status.Message)
		}
	} else {
		// Invalid action
		err = fmt.Errorf("invalid action %s for Redfish operation", action)
		hostOp.Status.Status = topohubv1beta1.HostOperationStatusFailed
		hostOp.Status.Message = err.Error()
		return err
	}

	logger.Info(hostOp.Status.Message)
	return nil
}
