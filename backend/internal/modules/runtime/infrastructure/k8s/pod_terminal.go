package k8s

import (
	"bytes"
	"context"
	"fmt"
	"strings"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/remotecommand"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/runtime/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/runtime/domain/port"
	platformk8s "github.com/sparkops/sparkops-agent/backend/internal/platform/k8s"
)

type PodTerminal struct {
	client    kubernetes.Interface
	restCfg   *rest.Config
}

func NewPodTerminal(factory *platformk8s.Factory, cluster string) (*PodTerminal, error) {
	if factory == nil {
		return nil, fmt.Errorf("k8s factory is required")
	}
	client, err := factory.Get(cluster)
	if err != nil {
		return nil, err
	}
	restCfg, err := factory.RESTConfig(cluster)
	if err != nil {
		return nil, err
	}
	return &PodTerminal{client: client, restCfg: restCfg}, nil
}

func NewPodTerminalWithClient(client kubernetes.Interface, restCfg *rest.Config) *PodTerminal {
	return &PodTerminal{client: client, restCfg: restCfg}
}

func (t *PodTerminal) StreamLines(ctx context.Context, pod *model.Pod) []string {
	if err := ValidatePod(pod); err != nil {
		return []string{"# error: " + err.Error()}
	}
	output, err := t.exec(ctx, pod, []string{
		"/bin/sh", "-c", "echo connected; whoami; hostname; ps aux 2>/dev/null | head -n 5 || ps -ef | head -n 5",
	})
	if err != nil {
		return []string{
			fmt.Sprintf("# kubectl exec -it %s -n %s -c %s -- /bin/sh", pod.Name, pod.Namespace, pod.Container),
			"# exec failed: " + err.Error(),
		}
	}
	lines := strings.Split(strings.TrimSpace(output), "\n")
	prefix := fmt.Sprintf("# connected to %s/%s", pod.Namespace, pod.Name)
	return append([]string{prefix}, lines...)
}

func (t *PodTerminal) exec(ctx context.Context, pod *model.Pod, command []string) (string, error) {
	req := t.client.CoreV1().RESTClient().Post().
		Resource("pods").
		Name(pod.Name).
		Namespace(pod.Namespace).
		SubResource("exec").
		VersionedParams(&corev1.PodExecOptions{
			Container: pod.Container,
			Command:   command,
			Stdout:    true,
			Stderr:    true,
		}, scheme.ParameterCodec)

	executor, err := remotecommand.NewSPDYExecutor(t.restCfg, "POST", req.URL())
	if err != nil {
		return "", fmt.Errorf("create executor: %w", err)
	}

	var stdout, stderr bytes.Buffer
	err = executor.StreamWithContext(ctx, remotecommand.StreamOptions{
		Stdout: &stdout,
		Stderr: &stderr,
	})
	if err != nil {
		if stderr.Len() > 0 {
			return "", fmt.Errorf("%w: %s", err, strings.TrimSpace(stderr.String()))
		}
		return "", err
	}
	if stderr.Len() > 0 && stdout.Len() == 0 {
		return strings.TrimSpace(stderr.String()), nil
	}
	return strings.TrimSpace(stdout.String()), nil
}

var _ port.PodTerminal = (*PodTerminal)(nil)
