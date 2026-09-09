import type {
  DeploymentOverviewVM,
  PodVM,
  ServiceOverviewVM,
} from "@sparkops/contracts/cluster";
import { stringifyYamlEditorDocument } from "@/modules/plugins/kubernetes/yaml-document";

function parseTargetPort(value: string): number | string {
  const trimmed = value.trim();
  const parsed = Number(trimmed);
  if (Number.isFinite(parsed) && String(parsed) === trimmed) {
    return parsed;
  }
  return value;
}

export function buildServiceYaml(service: ServiceOverviewVM): string {
  const metadata: Record<string, unknown> = {
    name: service.name,
    namespace: service.namespace,
  };
  if (service.labels && Object.keys(service.labels).length > 0) {
    metadata.labels = service.labels;
  }
  if (service.annotations && Object.keys(service.annotations).length > 0) {
    metadata.annotations = service.annotations;
  }

  const doc = {
    apiVersion: "v1",
    kind: "Service",
    metadata,
    spec: {
      selector: service.selector ?? {},
      ports: (service.ports ?? []).map((port) => {
        const item: Record<string, unknown> = {
          port: port.port,
          targetPort: parseTargetPort(port.target_port),
          protocol: port.protocol,
        };
        if (port.name) {
          item.name = port.name;
        }
        return item;
      }),
    },
  };

  return stringifyYamlEditorDocument(doc);
}

export function buildDeploymentYaml(
  deployment: DeploymentOverviewVM,
  pod?: PodVM | null,
): string {
  const appLabel = deployment.labels?.app ?? deployment.name;
  const matchLabels = deployment.labels?.app
    ? { app: deployment.labels.app }
    : { app: appLabel };
  const containerName = pod?.container || appLabel;
  const image =
    deployment.image || pod?.image || `${deployment.name}:${deployment.image_tag || "latest"}`;

  const container: Record<string, unknown> = {
    name: containerName,
    image,
  };
  if (deployment.memory_limit) {
    container.resources = {
      limits: { memory: deployment.memory_limit },
    };
  }

  const metadata: Record<string, unknown> = {
    name: deployment.name,
    namespace: deployment.namespace,
  };
  if (deployment.labels && Object.keys(deployment.labels).length > 0) {
    metadata.labels = deployment.labels;
  }

  const templateLabels = deployment.labels?.app
    ? { app: deployment.labels.app }
    : (deployment.labels ?? { app: appLabel });

  const doc = {
    apiVersion: "apps/v1",
    kind: "Deployment",
    metadata,
    spec: {
      replicas: deployment.replicas,
      strategy: { type: deployment.strategy || "RollingUpdate" },
      selector: { matchLabels },
      template: {
        metadata: { labels: templateLabels },
        spec: {
          containers: [container],
        },
      },
    },
  };

  return stringifyYamlEditorDocument(doc);
}

export function buildPodYaml(pod: PodVM): string {
  const metadata: Record<string, unknown> = {
    name: pod.name,
    namespace: pod.namespace,
  };

  const container: Record<string, unknown> = {
    name: pod.container || pod.name,
    image: pod.image,
  };

  const spec: Record<string, unknown> = {
    containers: [container],
  };
  if (pod.node) {
    spec.nodeName = pod.node;
  }

  const status: Record<string, unknown> = {
    phase: pod.phase,
  };
  if (pod.started_at) {
    status.startTime = pod.started_at;
  }

  const doc = {
    apiVersion: "v1",
    kind: "Pod",
    metadata,
    spec,
    status,
  };

  return stringifyYamlEditorDocument(doc);
}
