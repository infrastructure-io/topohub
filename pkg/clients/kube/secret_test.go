package kube

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestGetAuthenticationSecret(t *testing.T) {
	const name, namespace = "test-auth", "test-namespace"
	ctx := context.TODO()
	reader := fakeSecretReader(&corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Data: map[string][]byte{
			dataKeyUsername: []byte("admin"),
			dataKeyPassword: []byte("123456"),
			dataKeySSHKey:   []byte("<ssh-key>"),
		},
	})

	t.Run("not found", func(t *testing.T) {
		_, err := GetAuthenticationSecret(ctx, reader, "not-found", namespace)
		require.ErrorContains(t, err, "failed to get authentication secret test-namespace/not-found")
	})

	t.Run("get successful", func(t *testing.T) {
		auth, err := GetAuthenticationSecret(ctx, reader, name, namespace)
		require.NoError(t, err)
		assert.Equal(t, "admin", auth.Username)
		assert.Equal(t, "123456", auth.Password)
		assert.Equal(t, "<ssh-key>", auth.SSHKey)
	})
}

func fakeSecretReader(objs ...client.Object) client.Reader {
	scheme := runtime.NewScheme()
	scheme.AddKnownTypes(corev1.SchemeGroupVersion,
		&corev1.Secret{},
		&corev1.SecretList{},
	)
	return fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(objs...).
		WithStatusSubresource(objs...).
		Build()
}
