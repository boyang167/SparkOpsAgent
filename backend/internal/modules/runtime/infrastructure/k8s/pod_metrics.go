package k8s

import (
	"context"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/rest"
	metricsclientset "k8s.io/metrics/pkg/client/clientset/versioned"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/runtime/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/runtime/domain/port"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

func fetchPodMetricsByName(
	ctx context.Context,
	restCfg *rest.Config,
	namespace string,
) (map[string]port.PodMetrics, error) {
	if restCfg == nil {
		return nil, fmt.Errorf("REST config is required")
	}
	client, err := metricsclientset.NewForConfig(restCfg)
	if err != nil {
		return nil, fmt.Errorf("create metrics client: %w", err)
	}
	list, err := client.MetricsV1beta1().PodMetricses(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	out := make(map[string]port.PodMetrics, len(list.Items))
	for _, item := range list.Items {
		var cpuMilli, memoryBytes int64
		for _, container := range item.Containers {
			cpuMilli += container.Usage.Cpu().MilliValue()
			memoryBytes += container.Usage.Memory().Value()
		}
		out[item.Name] = port.PodMetrics{
			CPUMillicores: cpuMilli,
			MemoryBytes:   memoryBytes,
		}
	}
	return out, nil
}

func metricsForPods(
	metricsByName map[string]port.PodMetrics,
	pods []*model.Pod,
) map[kernel.PodID]port.PodMetrics {
	out := make(map[kernel.PodID]port.PodMetrics, len(pods))
	for _, pod := range pods {
		if pod == nil {
			continue
		}
		if usage, ok := metricsByName[pod.Name]; ok {
			out[pod.ID] = usage
		}
	}
	return out
}
