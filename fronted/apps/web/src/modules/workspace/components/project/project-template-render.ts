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

function sanitizeTemplateSegment(raw: string) {
  const s = raw.trim().toLowerCase();
  if (!s) return "na";
  let out = "";
  let lastDash = false;
  for (const ch of s) {
    if (/[a-z0-9]/.test(ch)) {
      out += ch;
      lastDash = false;
    } else if (ch === "-" && !lastDash) {
      out += "-";
      lastDash = true;
    } else if (/[_\s.]/.test(ch) && !lastDash) {
      out += "-";
      lastDash = true;
    }
  }
  const trimmed = out.replace(/^-+|-+$/g, "");
  return trimmed || "na";
}

function registryHostFromURL(raw?: string) {
  const trimmed = raw?.trim() ?? "";
  if (!trimmed) return "";
  try {
    const url = trimmed.includes("://") ? new URL(trimmed) : new URL(`http://${trimmed}`);
    return url.host || trimmed.replace(/^\/+|\/+$/g, "");
  } catch {
    return trimmed.replace(/^\/+|\/+$/g, "");
  }
}

function buildImagePullSecretName(
  orgSlug: string,
  workspaceSlug: string,
  registrySecretName: string,
  projectName: string,
) {
  const random4 = String(Math.floor(Math.random() * 10000)).padStart(4, "0");
  const parts = [
    sanitizeTemplateSegment(orgSlug),
    sanitizeTemplateSegment(workspaceSlug),
    sanitizeTemplateSegment(registrySecretName),
    sanitizeTemplateSegment(projectName),
    random4,
  ];
  return parts.join("-").slice(0, 253).replace(/^-+|-+$/g, "");
}

export interface TemplateRenderHints {
  orgSlug?: string;
  orgName?: string;
  workspaceSlug?: string;
  workspaceName?: string;
  registrySecretName?: string;
  registryURL?: string;
  gitSecretName?: string;
  gitCredentialId?: string;
  registryCredentialId?: string;
}

export function buildTemplateVars(
  project: ProjectVM,
  hints: TemplateRenderHints = {},
): Record<string, string> {
  const name = project.name || "app";
  const namespace = project.namespace?.trim() || "default";
  const ports = project.resources?.ports ?? [];
  const configEntries = parseProjectConfig(project.resources?.project_config).filter((e) => e.key.trim());
  const gitRefValue = project.git_ref_value?.trim() || "main";
  const cpuRequest = project.resources?.cpu_request || "500m";
  const cpuLimit = project.resources?.cpu_limit || "1000m";
  const memoryRequest = project.resources?.memory_request || "512Mi";
  const memoryLimit = project.resources?.memory_limit || "1Gi";

  const workspaceSlug =
    hints.workspaceSlug?.trim() || sanitizeTemplateSegment(project.namespace?.trim() || name);
  const orgSlug = hints.orgSlug?.trim() || "org";
  const registrySecretName = hints.registrySecretName?.trim() || "registry";
  const registryHost = registryHostFromURL(hints.registryURL) || "harbor.local";
  const containerImage = `${registryHost}/${workspaceSlug}/${name}:${gitRefValue}`;
  const imagePullSecretName = buildImagePullSecretName(orgSlug, workspaceSlug, registrySecretName, name);

  const vars: Record<string, string> = {
    name,
    project_name: name,
    display_name: project.display_name?.trim() || name,
    namespace,
    language: project.language || "go",
    git_url: project.git_url ?? "",
    git_ref_type: project.git_ref_type || "branch",
    git_ref_value: gitRefValue,
    dockerfile_path: project.dockerfile_path || "./Dockerfile",
    cpu_request: cpuRequest,
    cpu_limit: cpuLimit,
    memory_request: memoryRequest,
    memory_limit: memoryLimit,
    "resources.requests.cpu": cpuRequest,
    "resources.requests.memory": memoryRequest,
    "resources.limits.cpu": cpuLimit,
    "resources.limits.memory": memoryLimit,
    replicas: String(project.resources?.replicas ?? 1),
    command: project.resources?.command ?? "",
    args: project.resources?.args ?? "",
    registry_type: project.resources?.registry_type || "Harbor",
    service_name: `${name}-service`,
    deployment_name: `${name}-deployment`,
    app_label: name,
    org_name: hints.orgName?.trim() || orgSlug,
    org_slug: orgSlug,
    workspace_name: hints.workspaceName?.trim() || workspaceSlug,
    workspace_slug: workspaceSlug,
    registry_secret_name: registrySecretName,
    registry_url: hints.registryURL?.trim() ?? "",
    registry_host: registryHost,
    image_pull_secret_name: imagePullSecretName,
    imagesecret: imagePullSecretName,
    image_secret_name: imagePullSecretName,
    imagePullSecretName: imagePullSecretName,
    ImagePullSecretName: imagePullSecretName,
    image: containerImage,
    image_name: containerImage,
    Image: containerImage,
    container_image: containerImage,
    memory: memoryRequest,
    maxMemory: memoryLimit,
    max_memory: memoryLimit,
    cpu: cpuRequest,
    max_cpu: cpuLimit,
    git_addr: project.git_url ?? "",
    git_version: gitRefValue,
    git_buildpath: project.dockerfile_path || "./Dockerfile",
    hub_addr: registryHost,
    hub_namespace: workspaceSlug,
    HubAddr: registryHost,
    HubNameSpace: workspaceSlug,
    ProjectName: name,
    CodeVersion: gitRefValue,
    git_credential_id: hints.gitCredentialId?.trim() || `git-${workspaceSlug}-${hints.gitSecretName?.trim() || "gitlab"}`,
    CodeCrendentialId: hints.gitCredentialId?.trim() || `git-${workspaceSlug}-${hints.gitSecretName?.trim() || "gitlab"}`,
    registry_credential_id: hints.registryCredentialId?.trim() || registrySecretName,
  };

  for (const entry of configEntries) {
    vars[entry.key] = entry.value;
  }

  vars.ports_yaml = formatPortsYaml(ports);
  vars.ports_json = JSON.stringify(ports);
  return vars;
}

function formatProjectEnvYaml(entries: Array<{ key: string; value: string }>) {
  return entries
    .filter((e) => e.key.trim())
    .sort((a, b) => a.key.localeCompare(b.key))
    .map(
      (e) =>
        `        - name: ${escapeYaml(e.key)}\n          value: ${escapeYaml(e.value)}`,
    )
    .join("\n");
}

function existingDeploymentEnvNames(yaml: string) {
  const names = new Set<string>();
  for (const match of yaml.matchAll(/^\s*-\s+name:\s+(\S+)/gm)) {
    names.add(match[1].replace(/^["']|["']$/g, ""));
  }
  return names;
}

export function injectProjectConfigEnv(yamlContent: string, projectConfig?: string) {
  const entries = parseProjectConfig(projectConfig).filter((e) => e.key.trim());
  if (entries.length === 0) return yamlContent;

  const existing = existingDeploymentEnvNames(yamlContent);
  const toAdd = entries.filter((e) => !existing.has(e.key.trim()));
  if (toAdd.length === 0) return yamlContent;

  const itemBlock = formatProjectEnvYaml(toAdd);
  const hasEnv = yamlContent.includes("\n        env:");
  const block = hasEnv ? itemBlock : `        env:\n${itemBlock}`;
  const anchors = ["\n        envFrom:", "\n        volumeMounts:", "\n        resources:"];
  for (const anchor of anchors) {
    const idx = yamlContent.indexOf(anchor);
    if (idx >= 0) {
      return `${yamlContent.slice(0, idx)}\n${block}${yamlContent.slice(idx)}`;
    }
  }

  if (hasEnv) {
    return `${yamlContent}\n${itemBlock}`;
  }
  return `${yamlContent}\n${block}\n`;
}

function formatPortsYaml(ports: ProjectPort[]) {
  if (ports.length === 0) {
    return `    - port: 8080
      name: http-80
      protocol: TCP`;
  }
  return ports
    .map((p) => {
      const container = p.container_port || 8080;
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

function buildTemplatePorts(ports: ProjectPort[]) {
  const items =
    ports.length > 0
      ? ports
      : [{ container_port: 8080, name: "http-80", protocol: "TCP" as const }];
  return items.map((p) => {
    const container = p.container_port || 8080;
    const service = p.service_port ?? container;
    const name = p.name?.trim() || `http-${service}`;
    const protocol = p.protocol || "TCP";
    return {
      port: service,
      name,
      protocol,
      targetPort: container,
      container_port: container,
      service_port: service,
    };
  });
}

function expandPortRangeBlocks(source: string, ports: ReturnType<typeof buildTemplatePorts>) {
  const re = /\{\{-?\s*range\s+\$k,\$v\s*:=\s*\.ports\s*\}\}([\s\S]*?)(?:\{\{-?\s*end\s*\}\}|(?=\{\{-?\s*if\s))/g;
  return source.replace(re, (_, block: string) =>
    ports
      .map((p) => {
        let line = block;
        for (const [key, val] of Object.entries(p)) {
          for (const token of [`{{$v.${key}}}`, `{{ $v.${key} }}`]) {
            line = line.split(token).join(String(val));
          }
        }
        return line;
      })
      .join(""),
  );
}

function expandResourceTypeConditionals(source: string, kind: GeneratedKind) {
  const re = /\{\{-?\s*if\s+eq\s+\.resourceType\s+"service"\s*\}\}/g;
  return kind === "Service" ? source.replace(re, "") : source.replace(re, "");
}

function renderTemplateSource(source: string, project: ProjectVM, kind: GeneratedKind) {
  const vars = buildTemplateVars(project);
  const ports = buildTemplatePorts(project.resources?.ports ?? []);
  let out = source;
  if (out.includes("{{")) {
    out = expandPortRangeBlocks(out, ports);
    out = expandResourceTypeConditionals(out, kind);
  }
  return renderPlaceholders(out, vars);
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

  const image = vars.image || `${vars.app_label}:latest`;
  const pullSecretBlock = vars.image_pull_secret_name
    ? `      imagePullSecrets:\n        - name: ${vars.image_pull_secret_name}\n`
    : "";

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
${pullSecretBlock}      containers:
        - name: ${vars.app_label}
          image: ${image}
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

function groovyQuote(value: string) {
  return value.replace(/'/g, "\\'");
}

function defaultJenkinsfile(vars: Record<string, string>, enableScan: boolean) {
  const scanStage = enableScan
    ? `
    stage('Sonar Scan') {
      steps {
        echo "SonarQube static scan enabled for ${groovyQuote(vars.name ?? "app")}"
      }
    }`
    : "";
  const defaultRepo = `${vars.registry_host}/${vars.workspace_slug}/${vars.name}`;
  return `pipeline {
  agent any

  options {
    disableConcurrentBuilds()
    timeout(time: 90, unit: 'MINUTES')
  }

  environment {
    APP_NAME               = '${groovyQuote(vars.name ?? "app")}'
    PROJECT_NAME           = '${groovyQuote(vars.name ?? "app")}'
    NAMESPACE              = '${groovyQuote(vars.namespace ?? "default")}'
    WORKSPACE_SLUG         = '${groovyQuote(vars.workspace_slug ?? "default")}'
    ORG_SLUG               = '${groovyQuote(vars.org_slug ?? "org")}'
    LANGUAGE               = '${groovyQuote(vars.language ?? "go")}'
    GIT_URL                = '${groovyQuote(vars.git_url ?? "")}'
    GIT_REF_TYPE           = '${groovyQuote(vars.git_ref_type ?? "branch")}'
    DEFAULT_GIT_REF        = '${groovyQuote(vars.git_ref_value ?? "main")}'
    DOCKERFILE_PATH        = '${groovyQuote(vars.dockerfile_path ?? "./Dockerfile")}'
    REGISTRY_HOST          = '${groovyQuote(vars.registry_host ?? "harbor.local")}'
    REGISTRY_TYPE          = '${groovyQuote(vars.registry_type ?? "Harbor")}'
    REGISTRY_URL           = '${groovyQuote(vars.registry_url ?? "")}'
    DEFAULT_IMAGE          = '${groovyQuote(vars.image ?? "")}'
    GIT_CREDENTIAL_ID      = '${groovyQuote(vars.git_credential_id ?? "gitlab")}'
    REGISTRY_CREDENTIAL_ID = '${groovyQuote(vars.registry_credential_id ?? "registry")}'
    IMAGE_PULL_SECRET      = '${groovyQuote(vars.image_pull_secret_name ?? "")}'
  }

  stages {
    stage('Prepare') {
      steps {
        script {
          env.BRANCH = (params.BRANCH ?: env.DEFAULT_GIT_REF ?: 'main')?.trim() ?: 'main'
          env.CODE_VERSION = (params.CodeVersion ?: env.BRANCH)?.trim() ?: env.BRANCH
          env.IMAGE_TAG = (params.IMAGE_TAG ?: 'latest')?.trim()
          env.IMAGE_REPOSITORY = (params.IMAGE_REPOSITORY ?: '${groovyQuote(defaultRepo)}')?.trim()
          env.FULL_IMAGE = "\${env.IMAGE_REPOSITORY}:\${env.IMAGE_TAG}"
          def registryURL = (env.REGISTRY_URL ?: '').trim()
          def registryHost = (env.REGISTRY_HOST ?: '').trim()
          def privateRegistryHost = registryHost ==~ /^(10\\.\\d{1,3}\\.\\d{1,3}\\.\\d{1,3}|192\\.168\\.\\d{1,3}\\.\\d{1,3}\\.\\d{1,3}|172\\.(1[6-9]|2\\d|3[01])\\.\\d{1,3}\\.\\d{1,3}|localhost|127\\.0\\.0\\.1)(:\\d+)?$/
          def harborInsecure = env.REGISTRY_TYPE == 'Harbor' && (
            registryURL.startsWith('http://') ||
            !registryURL.startsWith('https://') ||
            privateRegistryHost
          )
          env.REGISTRY_INSECURE = harborInsecure ? 'true' : 'false'
          if (params.ROLLBACK_OF?.trim()) {
            echo "Rollback build of #\${params.ROLLBACK_OF}"
          }
        }
        echo "SparkOps CI: \${env.APP_NAME} (\${env.LANGUAGE}) in \${env.NAMESPACE}"
        echo "Git \${env.GIT_URL} @ \${env.BRANCH} (\${env.GIT_REF_TYPE})"
        echo "Target image \${env.FULL_IMAGE}"
      }
    }

    stage('Checkout') {
      steps {
        script {
          def branch = (env.BRANCH ?: 'main').trim()
          def branchSpec = env.GIT_REF_TYPE == 'tag' ? "refs/tags/\${branch}" : "*/\${branch}"
          checkout([
            $class: 'GitSCM',
            branches: [[name: branchSpec]],
            extensions: [[$class: 'CloneOption', shallow: true, depth: 1]],
            userRemoteConfigs: [[url: env.GIT_URL, credentialsId: env.GIT_CREDENTIAL_ID]]
          ])
        }
      }
    }

    stage('Build') {
      steps {
        sh """
          set -e
          if ! command -v docker >/dev/null 2>&1; then
            echo "docker CLI not found on Jenkins agent"
            exit 1
          fi
          if ! docker info >/dev/null 2>&1; then
            echo "docker daemon not available (check /var/run/docker.sock mount)"
            exit 1
          fi
          docker build -f "\\$DOCKERFILE_PATH" -t "\${env.FULL_IMAGE}" .
        """
      }
    }
${scanStage}
    stage('Push') {
      steps {
        script {
          withCredentials([usernamePassword(credentialsId: env.REGISTRY_CREDENTIAL_ID, usernameVariable: 'REGISTRY_USER', passwordVariable: 'REGISTRY_PASS')]) {
            sh """
              set -e
              REG_HOST="\\$REGISTRY_HOST"
              if [ "\${env.REGISTRY_INSECURE}" = "true" ] && command -v openssl >/dev/null 2>&1; then
                CERT_DIR="/etc/docker/certs.d/\\${REG_HOST}"
                if mkdir -p "\\$CERT_DIR" 2>/dev/null; then
                  if [ ! -s "\\$CERT_DIR/ca.crt" ]; then
                    echo | openssl s_client -connect "\\${REG_HOST}:443" -servername "\\${REG_HOST}" 2>/dev/null \\\\
                      | sed -ne '/-BEGIN CERTIFICATE-/,/-END CERTIFICATE-/p' > "\\$CERT_DIR/ca.crt" || true
                  fi
                fi
              fi
              echo "\\$REGISTRY_PASS" | docker login "\\$REG_HOST" -u "\\$REGISTRY_USER" --password-stdin
              docker push "\${env.FULL_IMAGE}"
            """
          }
        }
      }
    }

    stage('Complete') {
      steps {
        echo "Image \${env.FULL_IMAGE} is ready; SparkOps will deploy to namespace \${env.NAMESPACE}"
      }
    }
  }

  post {
    success {
      echo "Build succeeded: \${env.FULL_IMAGE}"
    }
    failure {
      echo 'Build failed'
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

function collectTemplatePicks(project: ProjectVM, byId: Map<string, TemplateVM>) {
  const resolved = new Map<GeneratedKind, string>();

  const addPick = (kind: GeneratedKind, id?: string) => {
    const value = id?.trim();
    if (!value || resolved.has(kind)) return;
    resolved.set(kind, value);
  };

  const addByTemplateId = (id?: string) => {
    const value = id?.trim();
    if (!value) return;
    const template = byId.get(value);
    const kind = kindNorm(template?.kind) ?? "Deployment";
    addPick(kind, value);
  };

  addPick("Service", project.resources?.service_template_id);
  addPick("jenkinsFile", project.resources?.jenkinsfile_template_id);
  addPick("Deployment", project.resources?.deployment_template_id);
  addByTemplateId(project.template_id);

  const picks: Array<{ kind: GeneratedKind; templateId?: string }> = [];
  if (resolved.has("Service")) {
    picks.push({ kind: "Service", templateId: resolved.get("Service") });
  } else if (resolved.has("Deployment")) {
    picks.push({ kind: "Service", templateId: undefined });
  }
  if (resolved.has("jenkinsFile")) {
    picks.push({ kind: "jenkinsFile", templateId: resolved.get("jenkinsFile") });
  }
  if (resolved.has("Deployment")) {
    picks.push({ kind: "Deployment", templateId: resolved.get("Deployment") });
  }

  if (picks.length === 0 && project.template_id?.trim()) {
    const template = byId.get(project.template_id);
    picks.push({ kind: kindNorm(template?.kind) ?? "Deployment", templateId: project.template_id });
  }
  return picks;
}

/** 与后端 GenerateArtifacts / generateProjectTemplates 一致的生成数量 */
export function countGeneratedTemplates(project: ProjectVM, templates: TemplateVM[] = []) {
  const byId = new Map(templates.map((t) => [t.id, t]));
  return collectTemplatePicks(project, byId).length;
}

export function generateProjectTemplates(
  project: ProjectVM,
  templates: TemplateVM[],
): GeneratedTemplateArtifact[] {
  const vars = buildTemplateVars(project);
  const byId = new Map(templates.map((t) => [t.id, t]));

  const picks = collectTemplatePicks(project, byId);

  // Stable UI order: Service → Jenkinsfile → Deployment
  const order: GeneratedKind[] = ["Service", "jenkinsFile", "Deployment"];
  picks.sort((a, b) => order.indexOf(a.kind) - order.indexOf(b.kind));

  return picks.map(({ kind, templateId }) => {
    const template = templateId ? byId.get(templateId) : undefined;
    const source = template?.detail?.trim() ?? "";
    const usedCustomTemplate = Boolean(source && looksLikeManifest(source));
    let content = usedCustomTemplate
      ? renderTemplateSource(source, project, kind)
      : defaultContent(kind, project, vars);
    if (usedCustomTemplate && kind === "Deployment") {
      content = injectProjectConfigEnv(content, project.resources?.project_config);
    }

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
