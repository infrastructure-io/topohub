package hostoperation

import (
	"fmt"
	"time"

	topohubv1beta1 "github.com/infrastructure-io/topohub/pkg/k8s/apis/topohub.infrastructure.io/v1beta1"
	"go.uber.org/zap"
	"golang.org/x/crypto/ssh"
)

// SSH operation constants
const (
	SSHActionReboot   = "reboot"
	SSHActionShutdown = "shutdown"
)

// performSSHOperation perform SSH operation
func (r *HostOperationController) performSSHOperation(
	hostEndpoint *topohubv1beta1.HostEndpoint,
	action string,
	username string,
	password string,
	hostOp *topohubv1beta1.HostOperation,
) error {
	// use controller logger
	l := r.log.With(
		zap.String("hostEndpoint", hostEndpoint.Name),
		zap.String("action", action),
	)

	// Create SSH client config
	port := 22 // Default SSH port
	if hostEndpoint.Spec.Port != nil {
		port = int(*hostEndpoint.Spec.Port)
	}

	// Configure SSH client
	sshConfig := &ssh.ClientConfig{
		User: username,
		Auth: []ssh.AuthMethod{
			ssh.Password(password),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         10 * time.Second,
	}

	// Create a new SSH connection
	addr := fmt.Sprintf("%s:%d", hostEndpoint.Spec.IPAddr, port)
	l.Infof("Creating new SSH connection to %s", addr)
	client, err := ssh.Dial("tcp", addr, sshConfig)
	if err != nil {
		l.Errorf("Failed to dial SSH server: %v", err)
		hostOp.Status.Status = topohubv1beta1.HostOperationStatusFailed
		hostOp.Status.Message = fmt.Sprintf("Failed to create SSH connection: %v", err)
		return err
	}
	// Ensure we close the connection when done
	defer func() {
		if err := client.Close(); err != nil {
			l.Errorf("Failed to close SSH client: %v", err)
		}
	}()

	// Create a new SSH session
	session, err := client.NewSession()
	if err != nil {
		l.Errorf("Failed to create SSH session: %v", err)
		hostOp.Status.Status = topohubv1beta1.HostOperationStatusFailed
		hostOp.Status.Message = fmt.Sprintf("Failed to create SSH session: %v", err)
		return err
	}
	defer func() {
		if err := session.Close(); err != nil {
			l.Errorf("Failed to close SSH session: %v", err)
		}
	}()

	// Determine the command based on the action
	var cmd string
	switch action {
	case topohubv1beta1.SSHCmdShutdown:
		cmd = "shutdown -h now"
	case topohubv1beta1.SSHCmdRestart:
		cmd = "reboot"
	default:
		err = fmt.Errorf("unsupported power action for SSH: %s", action)
		hostOp.Status.Status = topohubv1beta1.HostOperationStatusFailed
		hostOp.Status.Message = err.Error()
		return err
	}

	// Execute the command
	l.Infof("Executing SSH command: %s", cmd)
	output, err := session.CombinedOutput(cmd)
	if err != nil {
		l.Errorf("SSH command failed: %v, output: %s", err, string(output))
		hostOp.Status.Status = topohubv1beta1.HostOperationStatusFailed
		hostOp.Status.Message = fmt.Sprintf("SSH command failed: %v, output: %s", err, string(output))
		return err
	}

	// Update operation status on success
	hostOp.Status.Status = topohubv1beta1.HostOperationStatusSuccess
	hostOp.Status.Message = fmt.Sprintf("Successfully performed %s via SSH for %s", action, hostEndpoint.Spec.IPAddr)
	l.Info(hostOp.Status.Message)

	return nil
}
