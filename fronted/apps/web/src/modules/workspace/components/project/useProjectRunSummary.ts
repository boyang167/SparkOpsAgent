"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
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
  reload: () => void;
}

export interface UseProjectRunSummaryOptions {
  workspaceClusters?: EnvironmentClusterRef[] | null;
  workspaceClustersLoading?: boolean;
  workspaceClustersError?: string | null;
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

function buildSharedEnvironmentSummary(
  deploymentItems: DeploymentRunVM[],
  options?: UseProjectRunSummaryOptions,
): ProjectEnvironmentSummary {
  if (options?.workspaceClustersLoading) {
    return { items: [], loading: true, error: null };
  }
  if (options?.workspaceClustersError) {
    return { items: [], loading: false, error: options.workspaceClustersError };
  }
  return {
    items: buildEnvironmentDeployRows(options?.workspaceClusters ?? [], deploymentItems),
    loading: false,
    error: null,
  };
}

export function useProjectRunSummary(
  workspaceId: string,
  projectId: string,
  options?: UseProjectRunSummaryOptions,
): ProjectRunSummary {
  const useSharedClusters = options?.workspaceClusters !== undefined;
  const [deployment, setDeployment] = useState<ProjectDeploymentSummary>(DEPLOYMENT_INITIAL);
  const [build, setBuild] = useState<ProjectBuildSummary>(BUILD_INITIAL);
  const [optionsClusters, setOptionsClusters] = useState<EnvironmentClusterRef[] | null>(null);
  const [optionsClustersLoading, setOptionsClustersLoading] = useState(!useSharedClusters);
  const [optionsClustersError, setOptionsClustersError] = useState<string | null>(null);
  const [reloadKey, setReloadKey] = useState(0);

  useEffect(() => {
    let cancelled = false;
    setDeployment(DEPLOYMENT_INITIAL);
    setBuild(BUILD_INITIAL);
    setOptionsClusters(null);
    setOptionsClustersLoading(!useSharedClusters);
    setOptionsClustersError(null);

    void delivery
      .listDeployments(projectId, workspaceId)
      .then((page) => {
        if (cancelled) return;
        const releaseItems = page.items.filter((item) => item.run_type !== "undeploy");
        setDeployment({
          total: page.total - (page.items.length - releaseItems.length),
          latest: releaseItems[0] ?? null,
          items: page.items,
          loading: false,
          error: null,
        });
      })
      .catch((err) => {
        if (cancelled) return;
        setDeployment({
          total: null,
          latest: null,
          items: [],
          loading: false,
          error: err instanceof BffError ? err.message : "部署统计不可用",
        });
      });

    void delivery
      .listBuilds(projectId, workspaceId)
      .then((items) => {
        if (cancelled) return;
        setBuild({
          total: items.length,
          latest: items[0] ?? null,
          loading: false,
          error: null,
        });
      })
      .catch((err) => {
        if (cancelled) return;
        setBuild({
          total: null,
          latest: null,
          loading: false,
          error: err instanceof BffError ? err.message : "构建统计不可用",
        });
      });

    if (!useSharedClusters) {
      void delivery
        .getDeploymentOptions(projectId, workspaceId)
        .then((value) => {
          if (cancelled) return;
          setOptionsClusters(
            value.clusters.map((cluster) => ({
              id: cluster.id,
              cluster_name: cluster.cluster_name,
              is_build_env: cluster.is_build_env,
              priority_id: cluster.priority_id,
              available: cluster.available,
            })),
          );
          setOptionsClustersError(null);
        })
        .catch((err) => {
          if (cancelled) return;
          setOptionsClusters([]);
          setOptionsClustersError(
            err instanceof BffError ? err.message : "环境信息不可用",
          );
        })
        .finally(() => {
          if (!cancelled) setOptionsClustersLoading(false);
        });
    }

    return () => {
      cancelled = true;
    };
  }, [projectId, workspaceId, useSharedClusters, reloadKey]);

  const environments = useMemo((): ProjectEnvironmentSummary => {
    if (useSharedClusters) {
      return buildSharedEnvironmentSummary(deployment.items, options);
    }
    if (optionsClustersLoading) {
      return { items: [], loading: true, error: null };
    }
    if (optionsClustersError) {
      return { items: [], loading: false, error: optionsClustersError };
    }
    return {
      items: buildEnvironmentDeployRows(optionsClusters ?? [], deployment.items),
      loading: false,
      error: null,
    };
  }, [
    useSharedClusters,
    options?.workspaceClusters,
    options?.workspaceClustersLoading,
    options?.workspaceClustersError,
    optionsClustersLoading,
    optionsClustersError,
    optionsClusters,
    deployment.items,
  ]);

  const reload = useCallback(() => setReloadKey((value) => value + 1), []);
  return useMemo(
    () => ({ deployment, build, environments, reload }),
    [deployment, build, environments, reload],
  );
}
