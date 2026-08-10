"use client";

import { useState } from "react";
import { useRouter } from "next/navigation";
import { agent } from "@sparkops/web-sdk";
import { BffError } from "@sparkops/contracts";

interface PublishAgentButtonProps {
  agentId: string;
  workspaceId: string;
  versionId: string;
}

export function PublishAgentButton({ agentId, workspaceId, versionId }: PublishAgentButtonProps) {
  const router = useRouter();
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  async function onPublish() {
    setLoading(true);
    setError(null);
    try {
      await agent.publishAgent(agentId, workspaceId, versionId);
      router.refresh();
    } catch (err) {
      setError(err instanceof BffError ? err.message : "发布失败");
    } finally {
      setLoading(false);
    }
  }

  return (
    <div className="flex flex-col items-end gap-2">
      <button className="btn" type="button" onClick={onPublish} disabled={loading}>
        {loading ? "发布中..." : "发布 Agent"}
      </button>
      {error ? <p className="error text-right">{error}</p> : null}
    </div>
  );
}
