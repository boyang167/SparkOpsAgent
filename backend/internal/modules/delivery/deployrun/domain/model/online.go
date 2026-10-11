package model

import "github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"

func IsOnlineRun(run *DeploymentRun) bool {
	return run != nil && run.Status == StatusSucceeded && !run.IsUndeploy()
}

// LatestRunByCluster expects runs ordered by created_at DESC.
func LatestRunByCluster(runs []*DeploymentRun) map[kernel.ClusterInfoID]*DeploymentRun {
	latest := make(map[kernel.ClusterInfoID]*DeploymentRun, len(runs))
	for _, run := range runs {
		if run == nil {
			continue
		}
		if _, seen := latest[run.ClusterID]; seen {
			continue
		}
		latest[run.ClusterID] = run
	}
	return latest
}

// OnlineClusters expects runs ordered by created_at DESC.
func OnlineClusters(runs []*DeploymentRun) []kernel.ClusterInfoID {
	seen := make(map[kernel.ClusterInfoID]struct{}, len(runs))
	online := make([]kernel.ClusterInfoID, 0)
	for _, run := range runs {
		if run == nil {
			continue
		}
		if _, ok := seen[run.ClusterID]; ok {
			continue
		}
		seen[run.ClusterID] = struct{}{}
		if IsOnlineRun(run) {
			online = append(online, run.ClusterID)
		}
	}
	return online
}
