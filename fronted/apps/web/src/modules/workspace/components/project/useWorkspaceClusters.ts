"use client";

import { useEffect, useState } from "react";
import type { AuthorizedClusterVM } from "@sparkops/contracts/delivery";
import { platform } from "@sparkops/web-sdk";
import type { EnvironmentClusterRef } from "./project-environment-status";

function toEnvironmentClusterRef(cluster: AuthorizedClusterVM): EnvironmentClusterRef {
  return {
    id: cluster.id,
    cluster_name: cluster.cluster_name,
    is_build_env: cluster.is_build_env,
    priority_id: cluster.priority_id,
    available: cluster.available,
  };
}

export function useWorkspaceClusters(workspaceId: string) {
  const [clusters, setClusters] = useState<EnvironmentClusterRef[] | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    let cancelled = false;
    setLoading(true);
    setError(null);

    void platform
      .listAuthorizedWorkspaceClusters(workspaceId)
      .then((items) => {
        if (cancelled) return;
        setClusters(items.map(toEnvironmentClusterRef));
      })
      .catch(() => {
        if (cancelled) return;
        setClusters([]);
        setError("环境信息不可用");
      })
      .finally(() => {
        if (!cancelled) setLoading(false);
      });

    return () => {
      cancelled = true;
    };
  }, [workspaceId]);

  return { clusters, loading, error };
}
