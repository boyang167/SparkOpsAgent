package k8s

import (
	"context"
	"fmt"
	"io"
	"strings"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/runtime/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/runtime/domain/port"
	platformk8s "github.com/sparkops/sparkops-agent/backend/internal/platform/k8s"
)

const defaultLogTailLines int64 = 200

type PodLogReader struct {
	client kubernetes.Interface
}

func NewPodLogReader(factory *platformk8s.Factory, cluster string) (*PodLogReader, error) {
	if factory == nil {
		return nil, fmt.Errorf("k8s factory is required")
	}
	client, err := factory.Get(cluster)
	if err != nil {
		return nil, err
	}
	return &PodLogReader{client: client}, nil
}

func NewPodLogReaderWithClient(client kubernetes.Interface) *PodLogReader {
	return &PodLogReader{client: client}
}

func (r *PodLogReader) ReadLogs(ctx context.Context, pod *model.Pod) (string, error) {
	if err := ValidatePod(pod); err != nil {
		return "", err
	}
	req := r.client.CoreV1().Pods(pod.Namespace).GetLogs(pod.Name, &corev1.PodLogOptions{
		Container: pod.Container,
		TailLines: ptrInt64(defaultLogTailLines),
	})
	stream, err := req.Stream(ctx)
	if err != nil {
		return "", fmt.Errorf("stream pod logs: %w", err)
	}
	defer stream.Close()

	data, err := io.ReadAll(stream)
	if err != nil {
		return "", fmt.Errorf("read pod logs: %w", err)
	}
	return strings.TrimSpace(string(data)), nil
}

func ptrInt64(v int64) *int64 { return &v }

var _ port.PodLogReader = (*PodLogReader)(nil)
