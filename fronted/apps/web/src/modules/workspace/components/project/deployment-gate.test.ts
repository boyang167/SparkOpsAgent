import { describe, expect, it } from "vitest";
import type { DeploymentOptionsVM, DeploymentRunVM } from "@sparkops/contracts/delivery";
import type { SecretVM } from "@sparkops/contracts/secret";
import type { ProjectVM } from "@sparkops/contracts/workspace";
import { deploymentEntryAction, deploymentGate, releaseEntryAction } from "./deployment-gate";

function approvedProject(patch: Partial<ProjectVM> = {}): ProjectVM {
  return {
    id: "p",
    name: "project",
    description: "",
    status: "active",
    review_status: "approved",
    review_comment: "",
    can_submit_review: false,
    can_review: false,
    secret_id: "git",
    jenkins_secret_id: "jenkins",
    kubernetes_secret_id: "kubernetes",
    resources: {
      registry_secret_id: "registry",
      sonarqube_secret_id: undefined,
      enable_static_scan: false,
    },
    ...patch,
  };
}

function boundSecrets(): SecretVM[] {
  return [
    { id: "git", name: "git", service_type: "GitLab", service_url: "" },
    { id: "jenkins", name: "jenkins", service_type: "Jenkins", service_url: "" },
    { id: "kubernetes", name: "kubernetes", service_type: "Kubernetes", service_url: "" },
    { id: "registry", name: "registry", service_type: "Harbor", service_url: "" },
    { id: "sonar", name: "sonar", service_type: "SonarQube", service_url: "" },
  ];
}

const options: DeploymentOptionsVM = {
  clusters: [{ id: "c", cluster_name: "dev", server: "https://k8s", priority_id: 0, is_build_env: false, available: true }],
  missing: [],
};

describe("deploymentGate", () => {
  it("requires Sonarqube only when static scan is enabled", () => {
    expect(deploymentGate(approvedProject(), boundSecrets(), options, null).allowed).toBe(true);

    const project = approvedProject({
      resources: {
        ...approvedProject().resources,
        enable_static_scan: true,
      },
    });
    expect(deploymentGate(project, boundSecrets(), options, null).missing).toContain(
      "Sonarqube 密钥",
    );
  });

  it("disables deploy without an authorized cluster", () => {
    const result = deploymentGate(
      approvedProject(),
      boundSecrets(),
      { clusters: [], missing: ["未授权集群"] },
      null,
    );

    expect(result.allowed).toBe(false);
    expect(result.missing).toContain("未授权集群");
  });

  it("requires approval and no active run", () => {
    const activeRun = {
      id: "run",
      status: "building",
    } as DeploymentRunVM;

    expect(
      deploymentGate(approvedProject({ review_status: "pending" }), boundSecrets(), options, null)
        .missing,
    ).toContain("项目审核通过");
    expect(deploymentGate(approvedProject(), boundSecrets(), options, activeRun).missing).toContain(
      "已有部署正在进行",
    );
  });
});

describe("deploymentEntryAction", () => {
  it("disables deployment while preflight is loading", () => {
    expect(
      deploymentEntryAction(approvedProject(), null, null, true, null),
    ).toEqual({
      disabled: true,
      label: "发布检查中…",
      reason: "正在检查发布条件",
    });
  });

  it("opens active deployment progress instead of creating another run", () => {
    const activeRun = { id: "run", status: "deploying" } as DeploymentRunVM;
    expect(deploymentEntryAction(approvedProject(), options, activeRun, false, null)).toEqual({
      disabled: false,
      label: "查看部署进度",
      reason: null,
    });
  });

  it("blocks unapproved projects and unavailable options", () => {
    expect(
      deploymentEntryAction(
        approvedProject({ review_status: "pending" }),
        { clusters: [], missing: ["缺少 Jenkins 密钥"] },
        null,
        false,
        null,
      ),
    ).toMatchObject({
      disabled: true,
      label: "发布",
      reason: "项目审核通过；缺少 Jenkins 密钥；未授权集群",
    });
  });
});

describe("releaseEntryAction", () => {
  it("opens the wizard without Jenkins or cluster secrets", () => {
    expect(releaseEntryAction(approvedProject(), null, false, null)).toEqual({
      disabled: false,
      label: "发布",
      reason: null,
    });
  });
});
