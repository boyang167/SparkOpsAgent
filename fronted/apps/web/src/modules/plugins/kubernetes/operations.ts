const base = "/api/bff/v1/plugins/kubernetes/clusters";

function segment(value: string) {
  return encodeURIComponent(value);
}

export function podOperationPath(clusterId: string, namespace: string, pod: string, operation: string) {
  return `${base}/${segment(clusterId)}/namespaces/${segment(namespace)}/pods/${segment(pod)}/${operation}`;
}

export function toCommandQuery(command: string[]) {
  const params = new URLSearchParams();
  command.forEach((arg) => params.append("command", arg));
  return params.toString();
}

export function cloudTTYProxyPath(clusterId: string, namespace: string, cloudShellName: string) {
  const service = `http:${cloudShellName}:7681`;
  return `${base}/${segment(clusterId)}/proxy/api/v1/namespaces/${segment(namespace)}/services/${segment(service)}/proxy/`;
}
