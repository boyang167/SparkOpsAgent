package k8s

import (
	"context"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/runtime/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/runtime/domain/port"
	platformk8s "github.com/sparkops/sparkops-agent/backend/internal/platform/k8s"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type PlatformPodMetricsReader struct {
	factory *platformk8s.Factory
	cluster string
}

func NewPlatformPodMetricsReader(factory *platformk8s.Factory, cluster string) (*PlatformPodMetricsReader, error) {
	if factory == nil {
		return nil, nil
	}
	return &PlatformPodMetricsReader{factory: factory, cluster: cluster}, nil
}

func (r *PlatformPodMetricsReader) MetricsForPods(
	ctx context.Context,
	_ kernel.ProjectID,
	pods []*model.Pod,
) (map[kernel.PodID]port.PodMetrics, error) {
	if r == nil || len(pods) == 0 {
		return map[kernel.PodID]port.PodMetrics{}, nil
	}
	restCfg, err := r.factory.RESTConfig(r.cluster)
	if err != nil {
		return map[kernel.PodID]port.PodMetrics{}, nil
	}
	namespace := pods[0].Namespace
	metricsByName, err := fetchPodMetricsByName(ctx, restCfg, namespace)
	if err != nil {
		return map[kernel.PodID]port.PodMetrics{}, nil
	}
	return metricsForPods(metricsByName, pods), nil
}

var _ port.PodMetricsReader = (*PlatformPodMetricsReader)(nil)

type MockPodMetricsReader struct{}

func NewMockPodMetricsReader() *MockPodMetricsReader {
	return &MockPodMetricsReader{}
}

func (m *MockPodMetricsReader) MetricsForPods(
	context.Context,
	kernel.ProjectID,
	[]*model.Pod,
) (map[kernel.PodID]port.PodMetrics, error) {
	return map[kernel.PodID]port.PodMetrics{}, nil
}

var _ port.PodMetricsReader = (*MockPodMetricsReader)(nil)
