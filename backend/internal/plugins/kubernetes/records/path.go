package records

import "fmt"

func ObjectPathFromPod(namespace, pod, container string) string {
	if namespace == "" || pod == "" {
		return ""
	}
	if container == "" {
		return fmt.Sprintf("namespaces/%s/pods/%s", namespace, pod)
	}
	return fmt.Sprintf("namespaces/%s/pods/%s/containers/%s", namespace, pod, container)
}
