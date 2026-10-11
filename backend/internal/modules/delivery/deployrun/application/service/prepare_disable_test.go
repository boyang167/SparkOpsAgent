package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	deployport "github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/deployrun/domain/port"
	deliveryport "github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/domain/port"
	jenkinsinfra "github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/infrastructure/jenkins"
)

func TestPrepareServiceDisableJobUsesProjectFolderAndName(t *testing.T) {
	mock := jenkinsinfra.NewMockExecutor()
	svc := NewPrepareService("-build", nil).UseJenkinsFactory(func(deployport.DeploymentSecret) (deliveryport.JenkinsExecutor, error) {
		return mock, nil
	})

	err := svc.DisableJob(context.Background(), deployport.ProjectDeploymentContext{Name: "orders", Namespace: "team-a"})

	require.NoError(t, err)
	require.Equal(t, "team-a/orders", mock.LastDisabledJob)
}
