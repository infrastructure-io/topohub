package kube

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	dataKeyUsername = "username"
	dataKeyPassword = "password"
	dataKeySSHKey   = "ssh-privatekey"
)

type AuthenticationSecret struct {
	Username string
	Password string
	SSHKey   string
}

// GetAuthenticationSecret returns username and password from the kubernetes secret
func GetAuthenticationSecret(ctx context.Context, reader client.Reader, name, namespace string,
) (*AuthenticationSecret, error) {
	var secret corev1.Secret
	objKey := client.ObjectKey{Name: name, Namespace: namespace}
	if err := reader.Get(ctx, objKey, &secret); err != nil {
		return nil, fmt.Errorf("failed to get authentication secret %s/%s, err: %v", namespace, name, err)
	}
	return &AuthenticationSecret{
		Username: string(secret.Data[dataKeyUsername]),
		Password: string(secret.Data[dataKeyPassword]),
		SSHKey:   string(secret.Data[dataKeySSHKey]),
	}, nil
}
