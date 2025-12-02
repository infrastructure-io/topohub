package hostoperation

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/infrastructure-io/topohub/pkg/clients/pool"
	"github.com/infrastructure-io/topohub/pkg/clients/redfish"
	"github.com/infrastructure-io/topohub/pkg/config"
	topohubv1beta1 "github.com/infrastructure-io/topohub/pkg/k8s/apis/topohub.infrastructure.io/v1beta1"
	logpkg "github.com/infrastructure-io/topohub/pkg/log"
)

// HostOperationController reconciles a HostOperation object
type HostOperationController struct {
	Scheme      *runtime.Scheme
	agentConfig *config.AgentConfig
	redfishPool pool.SessionPool[redfish.Client]
	log         *zap.SugaredLogger

	client.Client
}

func NewHostOperationController(mgr ctrl.Manager, agentConfig *config.AgentConfig) (*HostOperationController, error) {
	return &HostOperationController{
		Client:      mgr.GetClient(),
		Scheme:      mgr.GetScheme(),
		agentConfig: agentConfig,
		redfishPool: redfish.GetSessionPool(),
		log:         logpkg.Logger.Named("HostOperationController"),
	}, nil
}

// Reconcile is part of the main kubernetes reconciliation loop
func (r *HostOperationController) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := r.log.With("hostoperation", req.Name)
	logger.Debug("Starting reconcile for HostOperation")

	// get the HostOperation object
	var hostOp topohubv1beta1.HostOperation
	if err := r.Get(ctx, req.NamespacedName, &hostOp); err != nil {
		logger.Errorf("Failed to get HostOperation %s: %v", req.Name, err)
		return ctrl.Result{}, nil
	}

	var isSSHOperation bool
	if hostOp.Spec.HostType == "ssh" {
		isSSHOperation = true
		logger.Debugf("Operation type is SSH for %s", hostOp.Spec.StatusName)
	} else if hostOp.Spec.HostType == "redfish" {
		isSSHOperation = false
		logger.Debugf("Operation type is Redfish for %s", hostOp.Spec.StatusName)
	} else {
		logger.Errorf("Invalid operation type: %s", hostOp.Spec.HostType)
		return ctrl.Result{}, nil
	}

	// check if the host operation is pending
	if hostOp.Status.Status == "" || hostOp.Status.Status == topohubv1beta1.HostOperationStatusPending {
		logger.Infof("Processing HostOperation %s : action=%s, statusName=%s, type=%s", hostOp.Name, hostOp.Spec.Action, hostOp.Spec.StatusName, hostOp.Spec.HostType)

		// update status
		hostOp.Status.Status = topohubv1beta1.HostOperationStatusPending
		hostOp.Status.LastUpdateTime = time.Now().UTC().Format(time.RFC3339)

		// get host endpoint
		var hostEndpoint *topohubv1beta1.HostEndpoint
		var err error

		if isSSHOperation {
			// get SSHStatus
			sshStatus := &topohubv1beta1.SSHStatus{}
			if err := r.Get(ctx, client.ObjectKey{Name: hostOp.Spec.StatusName}, sshStatus); err != nil {
				logger.Errorf("Failed to get SSHStatus %s: %v", hostOp.Spec.StatusName, err)
				return ctrl.Result{}, nil
			}

			// get host endpoint from SSHStatus
			hostEndpoint, err = r.getHostEndpointFromSSHStatus(sshStatus)
		} else {
			// get RedfishStatus
			redfishStatus := &topohubv1beta1.RedfishStatus{}
			if err := r.Get(ctx, client.ObjectKey{Name: hostOp.Spec.StatusName}, redfishStatus); err != nil {
				logger.Errorf("Failed to get RedfishStatus %s: %v", hostOp.Spec.StatusName, err)
				return ctrl.Result{}, nil
			}

			// get host endpoint from RedfishStatus
			hostEndpoint, err = r.getHostEndpoint(redfishStatus)
		}

		if err != nil {
			hostOp.Status.Status = topohubv1beta1.HostOperationStatusPending
			logger.Warnf("Failed to get connection info for %s: %v, retry later", hostOp.Spec.StatusName, err)
			return ctrl.Result{}, nil
		}

		logger.Debugf("get connect config %s: %+v", hostOp.Spec.StatusName, hostEndpoint)
		hostOp.Status.ClusterName = *hostEndpoint.Spec.ClusterName
		hostOp.Status.IpAddr = hostEndpoint.Spec.IPAddr

		// get secret data
		username, password, err := r.getSecretData(
			*hostEndpoint.Spec.SecretName,
			*hostEndpoint.Spec.SecretNamespace,
		)
		if err != nil {
			r.log.Errorf("Failed to get secret data for HostEndpoint %s: %v", hostEndpoint.Name, err)
			return ctrl.Result{}, nil
		}

		// perform operation
		logger.Infof("Processing %s operation for %s", hostOp.Spec.HostType, hostEndpoint.Name)

		if isSSHOperation {
			// perform SSH operation
			err = r.performSSHOperation(hostEndpoint, hostOp.Spec.Action, username, password, &hostOp)
		} else {
			// perform Redfish operation
			err = r.performRedfishOperation(hostEndpoint, hostOp.Spec.Action, username, password, &hostOp)
		}

		hostOp.Status.LastUpdateTime = time.Now().UTC().Format(time.RFC3339)
		if err != nil {
			logger.Errorf("Failed to operate %s: %v", hostOp.Spec.StatusName, err)
			hostOp.Status.Status = topohubv1beta1.HostOperationStatusFailed
			hostOp.Status.Message = err.Error()
		} else {
			logger.Infof("Performing %s operation on %s (%s)", hostOp.Spec.Action, hostOp.Spec.StatusName, hostOp.Spec.HostType)
			hostOp.Status.Status = topohubv1beta1.HostOperationStatusSuccess
		}

		// update HostOperation status
		if err := r.Status().Update(ctx, &hostOp); err != nil {
			logger.Errorf("Action has been done, but failed to update HostOperation status: %v", err)
			return ctrl.Result{}, nil
		}
		logger.Debugf("Successfully updated HostOperation %s status", hostOp.Name)

	} else {
		logger.Infof("HostOperation %s has been processed", hostOp.Name)
		return ctrl.Result{}, nil
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager
func (r *HostOperationController) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&topohubv1beta1.HostOperation{}).
		Complete(r)
}

func (r *HostOperationController) getHostEndpoint(redfishStatus *topohubv1beta1.RedfishStatus) (*topohubv1beta1.HostEndpoint, error) {
	// Use RedfishStatus name as HostEndpoint name
	hostEndpointName := redfishStatus.Name

	// get the HostEndpoint
	hostEndpoint := &topohubv1beta1.HostEndpoint{}
	if err := r.Get(context.TODO(), client.ObjectKey{Name: hostEndpointName}, hostEndpoint); err != nil {
		return nil, fmt.Errorf("failed to get hostendpoint %s: %v", hostEndpointName, err)
	}

	return hostEndpoint, nil
}

// getHostEndpointFromSSHStatus 从SSHStatus获取关联的HostEndpoint
func (r *HostOperationController) getHostEndpointFromSSHStatus(sshStatus *topohubv1beta1.SSHStatus) (*topohubv1beta1.HostEndpoint, error) {
	// Use SSHStatus name as HostEndpoint name
	hostEndpointName := sshStatus.Name

	// get the HostEndpoint
	hostEndpoint := &topohubv1beta1.HostEndpoint{}
	if err := r.Get(context.TODO(), client.ObjectKey{Name: hostEndpointName}, hostEndpoint); err != nil {
		return nil, fmt.Errorf("failed to get hostendpoint %s: %v", hostEndpointName, err)
	}

	return hostEndpoint, nil
}

func (r *HostOperationController) getSecretData(secretName, secretNamespace string) (string, string, error) {
	r.log.Debugf("Attempting to get secret data for %s/%s", secretNamespace, secretName)

	// 使用 controller-runtime client 获取 Secret
	var secret corev1.Secret
	objKey := client.ObjectKey{Name: secretName, Namespace: secretNamespace}
	if err := r.Get(context.TODO(), objKey, &secret); err != nil {
		r.log.Errorf("Failed to get secret %s/%s: %v", secretNamespace, secretName, err)
		return "", "", err
	}

	username := string(secret.Data["username"])
	password := string(secret.Data["password"])
	r.log.Debugf("Successfully retrieved secret data for %s/%s", secretNamespace, secretName)
	return username, password, nil
}

// performIPMIPXEBoot perform IPMI PXE boot
func (r *HostOperationController) performIPMIPXEBoot(ipAddr, username, password string) error {
	// set PXE boot via IPMI
	r.log.Infof("Setting PXE boot via IPMI for %s", ipAddr)

	// build set pxe boot command with options=efiboot to ensure proper PXE boot
	setPxeCmd := exec.Command("ipmitool", "-I", "lanplus", "-H", ipAddr, "-U", username, "-P", password, "chassis", "bootdev", "pxe", "options=efiboot")
	// print command (hide password)
	r.log.Infof("Executing command: ipmitool -I lanplus -H %s -U %s -P %s chassis bootdev pxe options=efiboot", ipAddr, username, password)

	// execute command
	output, err := setPxeCmd.CombinedOutput()
	if err != nil {
		r.log.Errorf("Failed to set PXE boot via IPMI: %v, output: %s", err, string(output))
		return fmt.Errorf("failed to set PXE boot via IPMI: %v", err)
	}
	r.log.Infof("Successfully set PXE boot via IPMI for %s: %s", ipAddr, strings.TrimSpace(string(output)))

	// perform power reset via IPMI
	r.log.Infof("Performing power reset via IPMI for %s", ipAddr)

	// build power reset command
	resetCmd := exec.Command("ipmitool", "-I", "lanplus", "-H", ipAddr, "-U", username, "-P", password, "power", "reset")
	// print command (hide password)
	r.log.Infof("Executing command: ipmitool -I lanplus -H %s -U %s -P %s power reset", ipAddr, username, password)

	// execute command
	output, err = resetCmd.CombinedOutput()
	if err != nil {
		r.log.Errorf("Failed to perform power reset via IPMI: %v, output: %s", err, string(output))
		return fmt.Errorf("failed to perform power reset via IPMI: %v", err)
	}
	r.log.Infof("Successfully performed power reset via IPMI for %s: %s", ipAddr, strings.TrimSpace(string(output)))

	return nil
}
