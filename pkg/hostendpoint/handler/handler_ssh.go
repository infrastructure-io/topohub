package handler

import (
	"context"
	"fmt"

	"go.uber.org/zap"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/infrastructure-io/topohub/pkg/clients/kube"
	"github.com/infrastructure-io/topohub/pkg/clients/pool"
	"github.com/infrastructure-io/topohub/pkg/clients/ssh"
	topohubv1beta1 "github.com/infrastructure-io/topohub/pkg/k8s/apis/topohub.infrastructure.io/v1beta1"
)

type SSHHostEndpointHandler struct {
	client      client.Client
	cacheReader client.Reader
	sessionPool pool.SessionPool[ssh.Client]
}

// Check SSHHostEndpointHandler implements the HostEndpointHandler
var _ HostEndpointHandler = (*SSHHostEndpointHandler)(nil)

func (h *SSHHostEndpointHandler) RefreshSession(
	ctx context.Context,
	hostEndpoint *topohubv1beta1.HostEndpoint,
	auth *kube.AuthenticationSecret,
	_logger *zap.SugaredLogger,
) error {
	if auth == nil {
		var err error
		auth, err = kube.GetAuthenticationSecret(ctx, h.cacheReader,
			*hostEndpoint.Spec.SecretName, *hostEndpoint.Spec.SecretNamespace)
		if err != nil {
			return err
		}
	}
	sessionCfg := &ssh.SSHSessionConfig{
		Username:   auth.Username,
		Password:   auth.Password,
		SSHKey:     auth.SSHKey,
		SSHKeyAuth: auth.SSHKey != "",
		IPAddr:     hostEndpoint.Spec.IPAddr,
		Port:       int(*hostEndpoint.Spec.Port),
	}
	if _, err := h.sessionPool.Refresh(sessionCfg.SessionID(), sessionCfg); err != nil && err != pool.ErrSessionNotFound {
		return err
	}
	return nil
}

func (h *SSHHostEndpointHandler) CreateStatusIfNotExist(
	ctx context.Context,
	hostEndpoint *topohubv1beta1.HostEndpoint,
	logger *zap.SugaredLogger,
) (bool, error) {
	name := hostEndpoint.Name
	var existingStatus topohubv1beta1.SSHStatus
	err := h.client.Get(ctx, client.ObjectKey{Name: name}, &existingStatus)
	if err == nil {
		logger.Infof("SSHStatus %s already exists, no need to create", name)
		return false, nil
	}
	if !apierrors.IsNotFound(err) {
		return false, fmt.Errorf("failed to get SSHStatus %s: %v", name, err)
	}

	// SSHStatus doesn't exist, create a new one
	logger.Debugf("Creating new SSHStatus %s", name)
	if err := h.client.Create(ctx, &topohubv1beta1.SSHStatus{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
			Labels: map[string]string{
				topohubv1beta1.LabelClientMode: topohubv1beta1.SSH,
			},
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion:         topohubv1beta1.APIVersion,
					Kind:               topohubv1beta1.KindHostEndpoint,
					Name:               hostEndpoint.Name,
					UID:                hostEndpoint.UID,
					Controller:         &[]bool{true}[0],
					BlockOwnerDeletion: &[]bool{true}[0],
				},
			},
		},
	}); err != nil {
		return false, err
	}
	return true, nil
}
