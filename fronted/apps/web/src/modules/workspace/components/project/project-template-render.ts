import type { ProjectPort, ProjectVM } from "@sparkops/contracts/workspace";
import type { TemplateVM } from "@sparkops/contracts/product";
import { parseProjectConfig } from "./project-utils";

export type GeneratedKind = "Service" | "Deployment" | "jenkinsFile";

export interface GeneratedTemplateArtifact {
  key: string;
  kind: GeneratedKind;
  title: string;
  filename: string;
  content: string;
  templateId?: string;
  templateName?: string;
}

function kindNorm(kind?: string): GeneratedKind | null {
  const k = (kind ?? "").toLowerCase();
  if (k === "service") return "Service";
  if (k === "deployment") return "Deployment";
  if (k === "jenkinsfile") return "jenkinsFile";
  return null;
}

function looksLikeManifest(text: string) {
  const sample = text.slice(0, 400).toLowerCase();
  return (
    sample.includes("apiversion:") ||
    sample.includes("kind:") ||
    sample.includes("pipeline {") ||
    sample.includes("pipeline{") ||
    sample.includes("#!groovy") ||
    sample.includes("stages {")
  );
}

function escapeYaml(value: string) {
  if (/[:#{}[\],&*?|>!%@`]/.test(value) || value.includes("\n") || value.includes('"')) {
    return JSON.stringify(value);
  }
  return value;
}

export function buildTemplateVars(project: ProjectVM): Record<string, string> {
  const name = project.name || "app";
  const namespace = project.namespace?.trim() || "default";
  const ports = project.resources?.ports ?? [];
  const configEntries = parseProjectConfig(project.resources?.project_config).filter((e) => e.key.trim());

  const vars: Record<string, string> = {
    name,
    project_name: name,
    display_name: project.display_name?.trim() || name,
    namespace,
    language: project.language || "go",
    git_url: project.git_url ?? "",
    git_ref_type: project.git_ref_type || "branch",
    git_ref_value: project.git_ref_value || "main",
    dockerfile_path: project.dockerfile_path || "./Dockerfile",
    cpu_request: project.resources?.cpu_request || "500m",
    cpu_limit: project.resources?.cpu_limit || "1000m",
    memory_request: project.resources?.memory_request || "512Mi",
    memory_limit: project.resources?.memory_limit || "1Gi",
    replicas: String(project.resources?.replicas ?? 1),
    command: project.resources?.command ?? "",
    args: project.resources?.args ?? "",
    registry_type: project.resources?.registry_type || "Harbor",
    service_name: `${name}-service`,
    deployment_name: `${name}-deployment`,
    app_label: name,
  };

  for (const entry of configEntries) {
    vars[entry.key] = entry.value;
  }

  vars.ports_yaml = formatPortsYaml(ports);
  vars.ports_json = JSON.stringify(ports);
  return vars;
}

function formatPortsYaml(ports: ProjectPort[]) {
  if (ports.length === 0) {
    return `    - port: 80
      name: http-80
      protocol: TCP`;
  }
  return ports
    .map((p) => {
      const container = p.container_port || 80;
      const service = p.service_port ?? container;
      const protocol = p.protocol || "TCP";
      const name = p.name?.trim() || `http-${service}`;
      return `    - port: ${service}
      targetPort: ${container}
      name: ${name}
      protocol: ${protocol}`;
    })
    .join("\n");
}

export function renderPlaceholders(source: string, vars: Record<string, string>) {
  return source
    .replace(/\{\{\s*\.?([a-zA-Z0-9_.-]+)\s*\}\}/g, (_, key: string) => vars[key] ?? vars[key.toLowerCase()] ?? "")
    .replace(/\$\{\s*([a-zA-Z0-9_.-]+)\s*\}/g, (_, key: string) => vars[key] ?? vars[key.toLowerCase()] ?? "")
    .replace(/%([A-Z0-9_]+)%/g, (_, key: string) => vars[key.toLowerCase()] ?? vars[key] ?? "");
}

function defaultService(vars: Record<string, string>) {
  return `apiVersion: v1
kind: Service
metadata:
  name: ${vars.service_name}
  namespace: ${vars.namespace}
  annotations:
    sidecar.istio.io/inject: "false"
  labels:
    app: ${vars.service_name}
spec:
  selector:
    app: ${vars.app_label}
  ports:
${vars.ports_yaml}
---
`;
}

function defaultDeployment(vars: Record<string, string>, project: ProjectVM) {
  const commandLines = (project.resources?.command ?? "")
    .split(/\r?\n/)
    .map((l) => l.trim())
    .filter(Boolean);
  const argLines = (project.resources?.args ?? "")
    .split(/\r?\n/)
    .map((l) => l.trim())
    .filter(Boolean);
  const configEntries = parseProjectConfig(project.resources?.project_config).filter((e) => e.key.trim());

  const commandBlock =
    commandLines.length > 0
      ? `        command:\n${commandLines.map((c) => `          - ${escapeYaml(c)}`).join("\n")}\n`
      : "";
  const argsBlock =
    argLines.length > 0
      ? `        args:\n${argLines.map((a) => `          - ${escapeYaml(a)}`).join("\n")}\n`
      : "";
  const envBlock =
    configEntries.length > 0
      ? `        env:\n${configEntries
          .map((e) => `          - name: ${escapeYaml(e.key)}\n            value: ${escapeYaml(e.value)}`)
          .join("\n")}\n`
      : "";

  const containerPorts =
    (project.resources?.ports ?? []).length > 0
      ? (project.resources?.ports ?? [])
          .map(
            (p) => `        - containerPort: ${p.container_port || 8080}
          name: ${p.name?.trim() || `port-${p.container_port || 8080}`}
          protocol: ${p.protocol || "TCP"}`,
          )
          .join("\n")
      : `        - containerPort: 8080
          name: http
          protocol: TCP`;

  return `apiVersion: apps/v1
kind: Deployment
metadata:
  name: ${vars.deployment_name}
  namespace: ${vars.namespace}
  labels:
    app: ${vars.app_label}
spec:
  replicas: ${vars.replicas}
  selector:
    matchLabels:
      app: ${vars.app_label}
  template:
    metadata:
      labels:
        app: ${vars.app_label}
    spec:
      containers:
        - name: ${vars.app_label}
          image: ${vars.app_label}:latest
${commandBlock}${argsBlock}${envBlock}          ports:
${containerPorts}
          resources:
            requests:
              cpu: ${vars.cpu_request}
              memory: ${vars.memory_request}
            limits:
              cpu: ${vars.cpu_limit}
              memory: ${vars.memory_limit}
---
`;
}

function defaultJenkinsfile(vars: Record<string, string>, enableScan: boolean) {
  const scanStage = enableScan
    ? `    stage('Sonar Scan') {
      steps {
        echo 'Static code scan for ${vars.name}'
      }
    }
`
    : "";
  return `pipeline {
  agent any
  environment {
    APP_NAME = '${vars.name}'
    GIT_URL = '${vars.git_url}'
    GIT_REF = '${vars.git_ref_value}'
    DOCKERFILE = '${vars.dockerfile_path}'
    REGISTRY_TYPE = '${vars.registry_type}'
    NAMESPACE = '${vars.namespace}'
  }
  stages {
    stage('Checkout') {
      steps {
        checkout scm
      }
    }
    stage('Build') {
      steps {
        echo "Building \${APP_NAME} from \${GIT_URL}#\${GIT_REF}"
        sh "docker build -f \${DOCKERFILE} -t \${APP_NAME}:latest ."
      }
    }
${scanStage}    stage('Push') {
      steps {
        echo "Push image to \${REGISTRY_TYPE}"
      }
    }
    stage('Deploy') {
      steps {
        echo "Deploy \${APP_NAME} to namespace \${NAMESPACE}"
      }
    }
  }
}
`;
}

function defaultContent(kind: GeneratedKind, project: ProjectVM, vars: Record<string, string>) {
  if (kind === "Service") return defaultService(vars);
  if (kind === "Deployment") return defaultDeployment(vars, project);
  return defaultJenkinsfile(vars, Boolean(project.resources?.enable_static_scan));
}

function filenameFor(kind: GeneratedKind, name: string) {
  if (kind === "jenkinsFile") return "Jenkinsfile";
  if (kind === "Service") return `${name}-service.yaml`;
  return `${name}-deployment.yaml`;
}

export function generateProjectTemplates(
  project: ProjectVM,
  templates: TemplateVM[],
): GeneratedTemplateArtifact[] {
  const vars = buildTemplateVars(project);
  const byId = new Map(templates.map((t) => [t.id, t]));

  const picks: Array<{ kind: GeneratedKind; templateId?: string }> = [];
  const serviceId = project.resources?.service_template_id?.trim();
  const deployId = project.resources?.deployment_template_id?.trim();
  const jenkinsId = project.resources?.jenkinsfile_template_id?.trim();

  if (serviceId) picks.push({ kind: "Service", templateId: serviceId });
  if (jenkinsId) picks.push({ kind: "jenkinsFile", templateId: jenkinsId });
  if (deployId) picks.push({ kind: "Deployment", templateId: deployId });

  // Fallback: if only legacy template_id is set
  if (picks.length === 0 && project.template_id?.trim()) {
    const t = byId.get(project.template_id);
    const kind = kindNorm(t?.kind) ?? "Deployment";
    picks.push({ kind, templateId: project.template_id });
  }

  // Stable UI order: Service → Jenkinsfile → Deployment
  const order: GeneratedKind[] = ["Service", "jenkinsFile", "Deployment"];
  picks.sort((a, b) => order.indexOf(a.kind) - order.indexOf(b.kind));

  return picks.map(({ kind, templateId }) => {
    const template = templateId ? byId.get(templateId) : undefined;
    const source = template?.detail?.trim() ?? "";
    const content =
      source && looksLikeManifest(source)
        ? renderPlaceholders(source, vars)
        : defaultContent(kind, project, vars);

    return {
      key: `${kind}-${templateId || "default"}`,
      kind,
      title: kind === "jenkinsFile" ? "Jenkinsfile" : kind,
      filename: filenameFor(kind, vars.name),
      content: content.endsWith("\n") ? content : `${content}\n`,
      templateId,
      templateName: template?.name,
    };
  });
}
