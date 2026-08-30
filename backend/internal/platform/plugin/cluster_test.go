package plugin_test

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	sparkplugin "github.com/sparkops/sparkops-agent/backend/internal/platform/plugin"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type fakeClusterCredentialSource struct {
	creds map[kernel.ClusterInfoID]sparkplugin.ClusterCredential
	err   error
}

func (s fakeClusterCredentialSource) ListAuthorized(context.Context, kernel.WorkspaceID) ([]sparkplugin.ClusterCredential, error) {
	if s.err != nil {
		return nil, s.err
	}
	out := make([]sparkplugin.ClusterCredential, 0, len(s.creds))
	for _, cred := range s.creds {
		out = append(out, cred)
	}
	return out, nil
}

func (s fakeClusterCredentialSource) GetAuthorized(_ context.Context, _ kernel.WorkspaceID, clusterID kernel.ClusterInfoID) (*sparkplugin.ClusterCredential, error) {
	if s.err != nil {
		return nil, s.err
	}
	cred, ok := s.creds[clusterID]
	if !ok {
		return nil, kernel.NewDomainError("PLUGIN_CLUSTER_FORBIDDEN", "cluster is not granted to workspace")
	}
	return &cred, nil
}

func TestClusterFactoryReturnsAuthorizedClientsAndMeta(t *testing.T) {
	clusterID := kernel.NewClusterInfoID()
	certPEM, keyPEM := testCertificate(t)
	cred := sparkplugin.ClusterCredential{
		ID:                       clusterID,
		Name:                     "prod",
		Server:                   "https://kubernetes.example",
		CertificateAuthorityData: certPEM,
		ClientCertificateData:    certPEM,
		ClientKeyData:            keyPEM,
	}
	factory := sparkplugin.NewFactory(fakeClusterCredentialSource{
		creds: map[kernel.ClusterInfoID]sparkplugin.ClusterCredential{clusterID: cred},
	})

	clientset, dynamicClient, restConfig, meta, err := factory.ForWorkspace(context.Background(), kernel.NewWorkspaceID(), clusterID)
	require.NoError(t, err)
	require.NotNil(t, clientset)
	require.NotNil(t, dynamicClient)
	require.NotNil(t, restConfig)
	require.Equal(t, "https://kubernetes.example", restConfig.Host)
	require.NotNil(t, meta)
	require.Equal(t, "prod", meta.Name)

	items, err := factory.ListMeta(context.Background(), kernel.NewWorkspaceID())
	require.NoError(t, err)
	require.Len(t, items, 1)
	require.Equal(t, sparkplugin.ClusterMeta{ID: clusterID.String(), Name: "prod", Server: "https://kubernetes.example"}, items[0])
}

func TestClusterFactoryReturnsForbiddenForUnauthorizedCluster(t *testing.T) {
	err := kernel.NewDomainError("PLUGIN_CLUSTER_FORBIDDEN", "cluster is not granted to workspace")
	factory := sparkplugin.NewFactory(fakeClusterCredentialSource{err: err})

	_, _, _, _, got := factory.ForWorkspace(context.Background(), kernel.NewWorkspaceID(), kernel.NewClusterInfoID())
	require.ErrorIs(t, got, err)
	domainErr, ok := kernel.IsDomainError(got)
	require.True(t, ok)
	require.Equal(t, "PLUGIN_CLUSTER_FORBIDDEN", domainErr.Code())
}

func TestClusterFactoryRejectsIncompleteCredentialsWithoutLeakingSecrets(t *testing.T) {
	clusterID := kernel.NewClusterInfoID()
	cred := sparkplugin.ClusterCredential{
		ID:                       clusterID,
		Name:                     "prod",
		Server:                   "https://kubernetes.example",
		CertificateAuthorityData: "SECRET_CA",
		ClientCertificateData:    "SECRET_CERT",
	}
	factory := sparkplugin.NewFactory(fakeClusterCredentialSource{
		creds: map[kernel.ClusterInfoID]sparkplugin.ClusterCredential{clusterID: cred},
	})

	_, _, _, _, err := factory.ForWorkspace(context.Background(), kernel.NewWorkspaceID(), clusterID)
	require.Error(t, err)
	domainErr, ok := kernel.IsDomainError(err)
	require.True(t, ok)
	require.Equal(t, "PLUGIN_CLUSTER_CREDENTIALS_INCOMPLETE", domainErr.Code())
	require.NotContains(t, err.Error(), "SECRET_CA")
	require.NotContains(t, err.Error(), "SECRET_CERT")
}

func testCertificate(t *testing.T) (string, string) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)

	template := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "sparkops-test"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment | x509.KeyUsageCertSign,
		IsCA:         true,
	}
	der, err := x509.CreateCertificate(rand.Reader, &template, &template, &key.PublicKey, key)
	require.NoError(t, err)
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	keyBytes, err := x509.MarshalECPrivateKey(key)
	require.NoError(t, err)
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyBytes})
	return string(certPEM), string(keyPEM)
}
