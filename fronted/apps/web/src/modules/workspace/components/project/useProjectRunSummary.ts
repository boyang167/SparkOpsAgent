"use client";

import { useEffect, useState } from "react";
import { BffError } from "@sparkops/contracts";
import type { BuildVM, DeploymentRunVM } from "@sparkops/contracts/delivery";
import { delivery } from "@sparkops/web-sdk";
import type { EnvironmentClusterRef } from "./project-environment-status";
import {
  buildEnvironmentDeployRows,
  type EnvironmentDeployRow,
} from "./project-environment-status";

export interface ProjectDeploymentSummary {
  total: number | null;
  latest: DeploymentRunVM | null;
  items: DeploymentRunVM[];
  loading: boolean;
  error: string | null;
}

export interface ProjectBuildSummary {
  total: number | null;
  latest: BuildVM | null;
  loading: boolean;
  error: string | null;
}

export interface ProjectEnvironmentSummary {
  items: EnvironmentDeployRow[];
  loading: boolean;
  error: string | null;
}

export interface ProjectRunSummary {
  deployment: ProjectDeploymentSummary;
  build: ProjectBuildSummary;
  environments: ProjectEnvironmentSummary;
}

const DEPLOYMENT_INITIAL: ProjectDeploymentSummary = {
  total: null,
  latest: null,
  items: [],
  loading: true,
  error: null,
};

const BUILD_INITIAL: ProjectBuildSummary = {
  total: null,
  latest: null,
  loading: true,
  error: null,
};

const ENVIRONMENT_INITIAL: ProjectEnvironmentSummary = {
  items: [],
  loading: true,
  error: null,
};

const INITIAL: ProjectRunSummary = {
  deployment: DEPLOYMENT_INITIAL,
  build: BUILD_INITIAL,
  environments: ENVIRONMENT_INITIAL,
};

export function useProjectRunSummary(workspaceId: string, projectId: string): ProjectRunSummary {
  const [summary, setSummary] = useState<ProjectRunSummary>(INITIAL);

  useEffect(() => {
    let cancelled = false;
    setSummary(INITIAL);

    void Promise.allSettled([
      delivery.listDeployments(projectId, workspaceId),
      delivery.listBuilds(projectId, workspaceId),
      delivery.getDeploymentOptions(projectId, workspaceId),
    ]).then(([deploymentResult, buildResult, optionsResult]) => {
      if (cancelled) return;

      const deployment =
        deploymentResult.status === "fulfilled"
          ? {
              total: deploymentResult.value.total,
              latest: deploymentResult.value.items[0] ?? null,
              items: deploymentResult.value.items,
              loading: false,
              error: null,
            }
          : {
              total: null,
              latest: null,
              items: [],
              loading: false,
              error:
                deploymentResult.reason instanceof BffError
                  ? deploymentResult.reason.message
                  : "部署统计不可用",
            };

      const build =
        buildResult.status === "fulfilled"
          ? {
              total: buildResult.value.length,
              latest: buildResult.value[0] ?? null,
              loading: false,
              error: null,
            }
          : {
              total: null,
              latest: null,
              loading: false,
              error:
                buildResult.reason instanceof BffError
                  ? buildResult.reason.message
                  : "构建统计不可用",
            };

      const clusters: EnvironmentClusterRef[] =
        optionsResult.status === "fulfilled"
          ? optionsResult.value.clusters.map((cluster) => ({
              id: cluster.id,
              cluster_name: cluster.cluster_name,
              is_build_env: cluster.is_build_env,
              priority_id: cluster.priority_id,
              available: cluster.available,
            }))
          : [];
      const clustersError =
        optionsResult.status === "rejected"
          ? optionsResult.reason instanceof BffError
            ? optionsResult.reason.message
            : "环境信息不可用"
          : null;

      const environments: ProjectEnvironmentSummary = {
        items: buildEnvironmentDeployRows(clusters, deployment.items),
        loading: false,
        error: clustersError,
      };

      setSummary({ deployment, build, environments });
    });

    return () => {
      cancelled = true;
    };
  }, [projectId, workspaceId]);

  return summary;
}
