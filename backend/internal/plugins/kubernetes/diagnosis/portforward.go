package diagnosis

import (
	"context"
	"errors"
	"fmt"
	"math/rand/v2"
	"net/http"
	"sync"
	"time"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/portforward"
	"k8s.io/client-go/transport/spdy"
)

const (
	arthasHTTPPort     = 8563
	arthasPortHexPattern = ":2173"
)

type portForwardHandle struct {
	stop     chan struct{}
	localPort int
}

var (
	portForwardMu sync.Mutex
	portForwards  = map[string]portForwardHandle{}
)

type PortForwardStarter interface {
	Start(ctx context.Context, client kubernetes.Interface, restCfg *rest.Config, namespace, pod string, remotePort int) (localPort int, stop func(), err error)
}

type KubernetesPortForwardStarter struct{}

func (KubernetesPortForwardStarter) Start(ctx context.Context, client kubernetes.Interface, restCfg *rest.Config, namespace, pod string, remotePort int) (int, func(), error) {
	if client == nil || restCfg == nil {
		return 0, nil, errors.New("kubernetes client unavailable")
	}
	localPort := randomLocalPort()
	url := client.CoreV1().RESTClient().Post().
		Resource("pods").
		Namespace(namespace).
		Name(pod).
		SubResource("portforward").
		URL()

	transport, upgrader, err := spdy.RoundTripperFor(restCfg)
	if err != nil {
		return 0, nil, err
	}
	dialer := spdy.NewDialer(upgrader, &http.Client{Transport: transport}, http.MethodPost, url)
	stopChan := make(chan struct{})
	readyChan := make(chan struct{})
	ports := []string{fmt.Sprintf("%d:%d", localPort, remotePort)}
	forwarder, err := portforward.New(dialer, ports, stopChan, readyChan, nil, nil)
	if err != nil {
		return 0, nil, err
	}
	go func() {
		_ = forwarder.ForwardPorts()
	}()

	select {
	case <-readyChan:
		return localPort, func() { close(stopChan) }, nil
	case <-ctx.Done():
		close(stopChan)
		return 0, nil, ctx.Err()
	case <-time.After(10 * time.Second):
		close(stopChan)
		return 0, nil, errors.New("port forward start timeout")
	}
}

func randomLocalPort() int {
	return rand.IntN(65535-1024) + 1024
}

func registerPortForward(id string, localPort int, stop func()) {
	portForwardMu.Lock()
	defer portForwardMu.Unlock()
	if existing, ok := portForwards[id]; ok {
		close(existing.stop)
	}
	stopChan := make(chan struct{})
	portForwards[id] = portForwardHandle{stop: stopChan, localPort: localPort}
	go func() {
		<-stopChan
		if stop != nil {
			stop()
		}
	}()
}

func stopPortForward(id string) {
	portForwardMu.Lock()
	defer portForwardMu.Unlock()
	if existing, ok := portForwards[id]; ok {
		close(existing.stop)
		delete(portForwards, id)
	}
}

func localPortFor(id string) (int, bool) {
	portForwardMu.Lock()
	defer portForwardMu.Unlock()
	handle, ok := portForwards[id]
	if !ok {
		return 0, false
	}
	return handle.localPort, true
}
