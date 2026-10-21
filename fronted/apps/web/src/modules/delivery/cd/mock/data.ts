import type {
  ClusterVM,
  FleetVM,
  PipelineDefinition,
  PipelineLayout,
  PipelineVM,
  ProfileVM,
  ReleasePreflightVM,
  StepCatalogEntry,
} from "../types";

export const MOCK_CLUSTERS: ClusterVM[] = [
  { id: "staging-hz-1", name: "staging-hangzhou-1", region: "cn-hz", env: "staging", ready: true },
  { id: "staging-hz-2", name: "staging-hangzhou-2", region: "cn-hz", env: "staging", ready: true },
  { id: "prod-bj-1", name: "prod-beijing-1", region: "cn-bj", env: "prod", ready: true },
  { id: "prod-bj-2", name: "prod-beijing-2", region: "cn-bj", env: "prod", ready: true },
  { id: "prod-sh-1", name: "prod-shanghai-1", region: "cn-sh", env: "prod", ready: true },
  { id: "prod-gz-1", name: "prod-guangzhou-1", region: "cn-gz", env: "prod", ready: false, reason: "缺少 L4 cluster patch" },
];

const STANDARD_JAVA_PROD: PipelineDefinition = {
  apiVersion: "cd.sparkops.io/v1",
  kind: "Pipeline",
  metadata: {
    name: "standard-java-prod",
    description: "构建 → Staging 验证 → 审批 → 生产分波部署",
  },
  spec: {
    parameters: [
      { name: "git_ref_value", type: "string", required: true, description: "Git 分支或 Tag" },
      { name: "image_tag", type: "string", required: false, description: "镜像 Tag，留空则使用 Git 引用值" },
      { name: "target_env", type: "string", required: true, default: "prod", description: "目标环境" },
    ],
    stages: [
      {
        id: "build",
        steps: [{ uses: "jenkins.build", with: { branch: "{{ params.git_ref_value }}" } }],
      },
      {
        id: "lock",
        needs: ["build"],
        steps: [{ uses: "artifact.lock" }],
      },
      {
        id: "deploy-staging",
        needs: ["lock"],
        steps: [
          {
            uses: "fleet.deploy",
            with: { fleet: "staging-all", strategy: "parallel", profile: "staging" },
          },
        ],
      },
      {
        id: "verify-staging",
        needs: ["deploy-staging"],
        steps: [{ uses: "k8s.wait_rollout" }, { uses: "http.smoke", with: { url: "{{ render.staging.url }}" } }],
      },
      {
        id: "approve-prod",
        needs: ["verify-staging"],
        steps: [
          {
            uses: "approval.manual",
            with: {
              approvers: ["role:ops-lead", "role:release-manager"],
              min_approvals: 1,
              timeout: "24h",
            },
          },
        ],
      },
      {
        id: "deploy-prod",
        needs: ["approve-prod"],
        steps: [
          {
            uses: "fleet.deploy",
            with: {
              fleet: "prod-cn",
              strategy: "wave",
              progressive: "canary",
              profile: "prod-cn",
              onFailure: "pause",
            },
          },
        ],
      },
    ],
  },
};

const STAGING_ONLY: PipelineDefinition = {
  apiVersion: "cd.sparkops.io/v1",
  kind: "Pipeline",
  metadata: { name: "staging-only", description: "仅 Staging 环境快速验证" },
  spec: {
    parameters: [
      { name: "git_ref_value", type: "string", required: true },
      { name: "image_tag", type: "string", required: false },
    ],
    stages: [
      { id: "build", steps: [{ uses: "jenkins.build" }] },
      {
        id: "deploy",
        needs: ["build"],
        steps: [{ uses: "fleet.deploy", with: { fleet: "staging-all", strategy: "parallel", profile: "staging" } }],
      },
      {
        id: "verify",
        needs: ["deploy"],
        steps: [{ uses: "k8s.wait_rollout" }, { uses: "http.smoke" }],
      },
    ],
  },
};

const HOTFIX: PipelineDefinition = {
  apiVersion: "cd.sparkops.io/v1",
  kind: "Pipeline",
  metadata: { name: "hotfix", description: "热修复：构建后双人审批，并行生产部署" },
  spec: {
    parameters: [
      { name: "git_ref_value", type: "string", required: true },
      { name: "image_tag", type: "string", required: false },
    ],
    stages: [
      { id: "build", steps: [{ uses: "jenkins.build" }] },
      {
        id: "approve",
        needs: ["build"],
        steps: [
          {
            uses: "approval.manual",
            with: { approvers: ["role:ops-lead", "role:release-manager"], min_approvals: 2 },
          },
        ],
      },
      {
        id: "deploy",
        needs: ["approve"],
        steps: [
          { uses: "fleet.deploy", with: { fleet: "prod-cn", strategy: "parallel", profile: "prod-cn" } },
        ],
      },
    ],
  },
};

const RELEASE_STAGING_THEN_PROD: PipelineDefinition = {
  apiVersion: "cd.sparkops.io/v1",
  kind: "Pipeline",
  metadata: {
    name: "release-staging-then-prod",
    description: "发布编排：Staging 并行 → 验证 → 审批 → 生产分波（不含构建）",
  },
  spec: {
    parameters: [{ name: "image_tag", type: "string", required: true, description: "已锁定的镜像 Tag" }],
    stages: [
      {
        id: "deploy-staging",
        steps: [
          { uses: "fleet.deploy", with: { fleet: "staging-all", strategy: "parallel", profile: "staging" } },
        ],
      },
      {
        id: "verify",
        needs: ["deploy-staging"],
        steps: [{ uses: "k8s.wait_rollout" }, { uses: "http.smoke" }],
      },
      {
        id: "approve-prod",
        needs: ["verify"],
        steps: [
          {
            uses: "approval.manual",
            with: { approvers: ["role:ops-lead", "role:release-manager"], min_approvals: 1 },
          },
        ],
      },
      {
        id: "deploy-prod",
        needs: ["approve-prod"],
        steps: [{ uses: "fleet.deploy", with: { fleet: "prod-cn", strategy: "wave", profile: "prod-cn" } }],
      },
    ],
  },
};

function defaultLayout(stageIds: string[]): PipelineLayout {
  const stages: PipelineLayout["stages"] = {};
  stageIds.forEach((id, index) => {
    stages[id] = { x: 40 + (index % 3) * 240, y: 40 + Math.floor(index / 3) * 140 };
  });
  return { stages };
}

export const MOCK_PIPELINES: PipelineVM[] = [
  {
    id: "pipe-release-staging-then-prod",
    name: "Staging 后生产分波",
    description: RELEASE_STAGING_THEN_PROD.metadata.description ?? "",
    status: "active",
    version: 1,
    definition: RELEASE_STAGING_THEN_PROD,
    layout: defaultLayout(RELEASE_STAGING_THEN_PROD.spec.stages.map((s) => s.id)),
    updatedAt: "2026-09-03T12:00:00Z",
  },
  {
    id: "pipe-standard-java-prod",
    name: "标准生产发布",
    description: STANDARD_JAVA_PROD.metadata.description ?? "",
    status: "active",
    version: 3,
    definition: STANDARD_JAVA_PROD,
    layout: defaultLayout(STANDARD_JAVA_PROD.spec.stages.map((s) => s.id)),
    updatedAt: "2026-09-01T10:00:00Z",
  },
  {
    id: "pipe-staging-only",
    name: "Staging 验证",
    description: STAGING_ONLY.metadata.description ?? "",
    status: "active",
    version: 1,
    definition: STAGING_ONLY,
    layout: defaultLayout(STAGING_ONLY.spec.stages.map((s) => s.id)),
    updatedAt: "2026-08-28T08:00:00Z",
  },
  {
    id: "pipe-hotfix",
    name: "热修复",
    description: HOTFIX.metadata.description ?? "",
    status: "draft",
    version: 1,
    definition: HOTFIX,
    layout: defaultLayout(HOTFIX.spec.stages.map((s) => s.id)),
    updatedAt: "2026-08-20T14:30:00Z",
  },
];

export const MOCK_FLEETS: FleetVM[] = [
  {
    id: "fleet-staging-all",
    name: "staging-all",
    description: "全部 Staging 集群并行部署",
    clusters: MOCK_CLUSTERS.filter((c) => c.env === "staging"),
    topology: {
      waves: [{ name: "all", clusters: ["staging-hz-1", "staging-hz-2"] }],
    },
    updatedAt: "2026-09-01T09:00:00Z",
  },
  {
    id: "fleet-prod-cn",
    name: "prod-cn",
    description: "中国区生产：金丝雀 → 主区域 → 边缘",
    clusters: MOCK_CLUSTERS.filter((c) => c.env === "prod"),
    topology: {
      waves: [
        { name: "canary", clusters: ["prod-bj-1"] },
        { name: "primary", clusters: ["prod-bj-2", "prod-sh-1"] },
        { name: "edge", clusters: ["prod-gz-1"] },
      ],
    },
    updatedAt: "2026-09-02T11:00:00Z",
  },
];

export const MOCK_PROFILES: ProfileVM[] = [
  {
    id: "profile-staging",
    name: "staging",
    description: "Staging 环境默认参数",
    env: "staging",
    values: {
      replicas: 1,
      ingress_host: "staging.example.com",
      log_level: "debug",
      cpu_request: "100m",
      memory_request: "256Mi",
    },
    patch: { spec: { replicas: 1 } },
    updatedAt: "2026-08-15T12:00:00Z",
  },
  {
    id: "profile-prod-cn",
    name: "prod-cn",
    description: "中国区生产环境",
    env: "prod",
    values: {
      replicas: 3,
      ingress_host: "api.example.com",
      log_level: "info",
      cpu_request: "500m",
      memory_request: "1Gi",
      hpa_min: 3,
      hpa_max: 20,
    },
    patch: { spec: { replicas: 3 } },
    updatedAt: "2026-09-01T08:00:00Z",
  },
  {
    id: "profile-prod-ap",
    name: "prod-ap",
    description: "亚太区生产环境",
    env: "prod",
    values: {
      replicas: 2,
      ingress_host: "ap.example.com",
      log_level: "info",
      cpu_request: "500m",
      memory_request: "1Gi",
    },
    patch: { spec: { replicas: 2 } },
    updatedAt: "2026-08-30T16:00:00Z",
  },
];

export const STEP_CATALOG: StepCatalogEntry[] = [
  {
    uses: "jenkins.build",
    label: "Jenkins 构建",
    category: "构建",
    description: "触发 Jenkins Job 并等待构建完成。发布向导选择「使用已有镜像」时会跳过",
    fields: [
      { key: "branch", label: "分支", type: "string", placeholder: "{{ params.git_ref_value }}" },
      { key: "job", label: "Job 名称", type: "string", placeholder: "默认使用项目配置" },
    ],
  },
  {
    uses: "artifact.lock",
    label: "锁定制品",
    category: "构建",
    description: "锁定 image digest，保证多集群部署同一制品",
    fields: [],
  },
  {
    uses: "fleet.deploy",
    label: "Fleet 部署",
    category: "部署",
    description: "按 Fleet 拓扑多集群部署",
    fields: [
      { key: "fleet", label: "Fleet", type: "select", required: true, options: MOCK_FLEETS.map((f) => ({ value: f.name, label: f.name })) },
      {
        key: "strategy",
        label: "策略",
        type: "select",
        options: [
          { value: "parallel", label: "并行" },
          { value: "sequential", label: "顺序" },
          { value: "wave", label: "分波" },
        ],
      },
      {
        key: "profile",
        label: "Profile",
        type: "select",
        options: MOCK_PROFILES.map((p) => ({ value: p.name, label: p.name })),
      },
      {
        key: "onFailure",
        label: "失败策略",
        type: "select",
        options: [
          { value: "abort", label: "中止" },
          { value: "pause", label: "暂停" },
          { value: "continue", label: "继续" },
        ],
      },
    ],
  },
  {
    uses: "k8s.wait_rollout",
    label: "等待 Rollout",
    category: "验证",
    description: "等待 Deployment/Rollout 就绪",
    fields: [{ key: "timeout", label: "超时", type: "string", placeholder: "10m" }],
  },
  {
    uses: "http.smoke",
    label: "HTTP 冒烟",
    category: "验证",
    description: "对服务端点发起 HTTP 检查",
    fields: [{ key: "url", label: "URL", type: "string", required: true }],
  },
  {
    uses: "approval.manual",
    label: "人工审批",
    category: "治理",
    description: "暂停 Pipeline，等待人工审批",
    pauses: true,
    fields: [
      { key: "approvers", label: "审批人", type: "string-list", placeholder: "role:ops-lead" },
      { key: "min_approvals", label: "最少批准数", type: "number" },
      { key: "timeout", label: "超时", type: "string", placeholder: "24h" },
    ],
  },
  {
    uses: "policy.evaluate",
    label: "策略评估",
    category: "治理",
    description: "执行 Policy Engine 规则",
    fields: [{ key: "policy_set", label: "策略集", type: "string" }],
  },
  {
    uses: "notify.webhook",
    label: "Webhook 通知",
    category: "通知",
    description: "发送 Webhook 通知",
    fields: [{ key: "url", label: "Webhook URL", type: "string" }],
  },
];

export function buildReleasePreflight(): ReleasePreflightVM {
  return {
    pipelines: MOCK_PIPELINES.filter((p) => p.status === "active").map((p) => ({
      id: p.id,
      name: p.name,
      version: p.version,
      description: p.description,
    })),
    fleets: MOCK_FLEETS.map((fleet) => ({
      id: fleet.id,
      name: fleet.name,
      clusters: fleet.clusters.map((cluster) => {
        const wave = fleet.topology.waves.find((w) => w.clusters.includes(cluster.id));
        return { ...cluster, wave: wave?.name };
      }),
    })),
    policies: [
      { name: "prod-change-window", passed: true },
      { name: "prod-gz-patch", passed: false, message: "prod-guangzhou-1 缺少 L4 patch" },
      { name: "forbid-latest-tag", passed: true },
    ],
    blocked: false,
  };
}

/** 简易内存 store，模拟 CRUD */
export function cloneMockPipelines(): PipelineVM[] {
  return structuredClone(MOCK_PIPELINES);
}

export function cloneMockFleets(): FleetVM[] {
  return structuredClone(MOCK_FLEETS);
}

export function cloneMockProfiles(): ProfileVM[] {
  return structuredClone(MOCK_PROFILES);
}

export function getMockPipeline(id: string): PipelineVM | undefined {
  return cloneMockPipelines().find((p) => p.id === id);
}

export function getMockFleet(id: string): FleetVM | undefined {
  return cloneMockFleets().find((f) => f.id === id);
}

export function getMockProfile(id: string): ProfileVM | undefined {
  return cloneMockProfiles().find((p) => p.id === id);
}

export function getStepCatalogEntry(uses: string): StepCatalogEntry | undefined {
  return STEP_CATALOG.find((s) => s.uses === uses);
}
