import { KubernetesApiError, parseApiError } from "./api";
import { serviceProxyPath } from "./catalog";
import { appendStreamEvent, createSSEParser, K8S_STREAM_LIMIT } from "./sse";

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
  return serviceProxyPath(clusterId, namespace, `http:${cloudShellName}:7681`);
}

export type NodeTTYPodStatus = {
  status?: {
    phase?: string;
    containerStatuses?: { name?: string; ready?: boolean }[];
  };
};

export function isNodeTTYReady(pod: NodeTTYPodStatus) {
  return pod.status?.phase === "Running" &&
    pod.status.containerStatuses?.some((item) => item.name === "node-shell" && item.ready === true) === true;
}

export async function waitForNodeTTYReady(options: {
  load: () => Promise<NodeTTYPodStatus>;
  signal: AbortSignal;
  attempts?: number;
  intervalMs?: number;
  sleep?: (ms: number, signal: AbortSignal) => Promise<void>;
  onAttempt?: (attempt: number, pod: NodeTTYPodStatus) => void;
}) {
  const attempts = options.attempts ?? 60;
  const intervalMs = options.intervalMs ?? 1000;
  const sleep = options.sleep ?? abortableSleep;
  for (let attempt = 0; attempt < attempts; attempt += 1) {
    if (options.signal.aborted) throw new Error("NodeTTY 等待已取消");
    const pod = await options.load();
    options.onAttempt?.(attempt + 1, pod);
    if (isNodeTTYReady(pod)) return pod;
    if (attempt + 1 < attempts) await sleep(intervalMs, options.signal);
  }
  throw new Error("NodeTTY Pod 就绪超时");
}

export async function readKubernetesSSE(
  url: string,
  setOutput: (updater: (current: string[]) => string[]) => void,
  signal?: AbortSignal,
) {
  const res = await fetch(url, { cache: "no-store", signal });
  if (!res.ok) throw new KubernetesApiError(await parseApiError(res));
  if (!res.body) throw new Error("SSE 响应为空");
  const reader = res.body.pipeThrough(new TextDecoderStream()).getReader();
  const parser = createSSEParser((event) => {
    setOutput((current) => appendStreamEvent(current, event, K8S_STREAM_LIMIT));
  });
  signal?.addEventListener("abort", () => void reader.cancel().catch(() => undefined), { once: true });
  try {
    for (;;) {
      const { value, done } = await reader.read();
      if (done) break;
      parser.push(value);
    }
    parser.flush();
  } finally {
    reader.releaseLock();
  }
}

function abortableSleep(ms: number, signal: AbortSignal) {
  return new Promise<void>((resolve, reject) => {
    const timer = window.setTimeout(resolve, ms);
    signal.addEventListener("abort", () => {
      window.clearTimeout(timer);
      reject(new Error("NodeTTY 等待已取消"));
    }, { once: true });
  });
}
