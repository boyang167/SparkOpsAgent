"use client";

import { useEffect, useState } from "react";
import type { GitTreeEntryVM } from "@sparkops/contracts/workspace";
import type { SecretVM } from "@sparkops/contracts/secret";
import { BffError } from "@sparkops/contracts";
import { workspace } from "@sparkops/web-sdk";
import { SelectOrManualField } from "./SelectOrManualField";
import type { BuildServiceRow } from "./build-job-spec";
import { isDockerfileName, toRepoRelativePath } from "./dockerfile-options";

interface BuildCodeRepoFieldsProps {
  workspaceId: string;
  row: BuildServiceRow;
  gitSecrets: SecretVM[];
  selectedSecretId: string;
  onPatch: (patch: Partial<BuildServiceRow>) => void;
}

async function collectDockerfiles(
  workspaceId: string,
  gitUrl: string,
  secretId: string,
  ref: string,
): Promise<string[]> {
  const found: string[] = [];
  const queue: { path: string; depth: number }[] = [{ path: "", depth: 0 }];
  let visited = 0;
  while (queue.length > 0 && visited < 24) {
    const current = queue.shift();
    if (!current) break;
    visited += 1;
    let items: GitTreeEntryVM[] = [];
    try {
      items = await workspace.listGitTree(workspaceId, {
        git_url: gitUrl,
        secret_id: secretId,
        ref,
        path: current.path,
      });
    } catch {
      continue;
    }
    for (const item of items) {
      if (item.type === "blob" && isDockerfileName(item.name)) {
        found.push(toRepoRelativePath(item.path));
      } else if (item.type === "tree" && current.depth < 3) {
        queue.push({ path: item.path, depth: current.depth + 1 });
      }
    }
  }
  return [...new Set(found)].sort();
}

export function BuildCodeRepoFields({
  workspaceId,
  row,
  gitSecrets,
  selectedSecretId,
  onPatch,
}: BuildCodeRepoFieldsProps) {
  const secret = gitSecrets.find((item) => item.id === selectedSecretId);
  const gitServiceType = (secret?.service_type || "GitLab").toLowerCase() === "github" ? "GitHub" : "GitLab";
  const refType = row.git_ref_type === "tag" ? "tag" : "branch";
  const [refOptions, setRefOptions] = useState<string[]>([]);
  const [tagNames, setTagNames] = useState<string[]>([]);
  const [refHint, setRefHint] = useState<string | null>(null);
  const [loadingRefs, setLoadingRefs] = useState(false);
  const [dockerfiles, setDockerfiles] = useState<string[]>([]);
  const [dockerHint, setDockerHint] = useState<string | null>(null);
  const [loadingDocker, setLoadingDocker] = useState(false);

  useEffect(() => {
    const gitUrl = row.git_url.trim();
    const secretId = selectedSecretId.trim();
    if (!workspaceId || !gitUrl || !secretId || gitServiceType !== "GitLab") {
      setRefOptions([]);
      setRefHint(
        !secretId
          ? "请先选择仓库密钥"
          : gitServiceType !== "GitLab"
            ? "当前仅 GitLab 支持从仓库加载列表，可改用手动输入"
            : null,
      );
      return;
    }
    let cancelled = false;
    const timer = window.setTimeout(() => {
      setLoadingRefs(true);
      setRefHint(null);
      void Promise.all([
        workspace.listGitRefs(workspaceId, { git_url: gitUrl, secret_id: secretId, ref_type: "branch" }),
        workspace.listGitRefs(workspaceId, { git_url: gitUrl, secret_id: secretId, ref_type: "tag" }),
      ])
        .then(([branches, tags]) => {
          if (cancelled) return;
          const tagSet = new Set(tags.map((item) => item.name));
          setTagNames([...tagSet]);
          const names = (refType === "tag" ? tags : branches)
            .map((item) => item.name)
            .filter((name) => (refType === "branch" ? !tagSet.has(name) : true));
          setRefOptions(names);
        })
        .catch((err: unknown) => {
          if (cancelled) return;
          setRefOptions([]);
          setRefHint(err instanceof BffError ? err.message : "加载分支/标签失败");
        })
        .finally(() => {
          if (!cancelled) setLoadingRefs(false);
        });
    }, 300);
    return () => {
      cancelled = true;
      window.clearTimeout(timer);
    };
  }, [workspaceId, row.git_url, selectedSecretId, gitServiceType, refType]);

  useEffect(() => {
    const gitUrl = row.git_url.trim();
    const secretId = selectedSecretId.trim();
    const ref = row.branch.trim();
    if (!workspaceId || !gitUrl || !secretId || !ref || gitServiceType !== "GitLab") {
      setDockerfiles(row.dockerfile_path ? [row.dockerfile_path] : ["./Dockerfile"]);
      setDockerHint(ref ? null : "请先选择分支或标签");
      return;
    }
    let cancelled = false;
    const timer = window.setTimeout(() => {
      setLoadingDocker(true);
      setDockerHint(null);
      void collectDockerfiles(workspaceId, gitUrl, secretId, ref)
        .then((items) => {
          if (!cancelled) setDockerfiles(items.length ? items : ["./Dockerfile"]);
        })
        .catch((err: unknown) => {
          if (cancelled) return;
          setDockerfiles(["./Dockerfile"]);
          setDockerHint(err instanceof BffError ? err.message : "加载 Dockerfile 失败");
        })
        .finally(() => {
          if (!cancelled) setLoadingDocker(false);
        });
    }, 300);
    return () => {
      cancelled = true;
      window.clearTimeout(timer);
    };
  }, [workspaceId, row.git_url, selectedSecretId, row.branch, gitServiceType, row.dockerfile_path]);

  return (
    <>
      <div className="field">
        <span className="text-xs text-muted">引用类型</span>
        <div className="pwf-radio-row" role="radiogroup" aria-label="引用类型">
          <label className="pwf-radio">
            <input
              type="radio"
              name="pwf-git-ref-type"
              checked={refType === "branch"}
              onChange={() => onPatch({ git_ref_type: "branch" })}
            />
            分支
          </label>
          <label className="pwf-radio">
            <input
              type="radio"
              name="pwf-git-ref-type"
              checked={refType === "tag"}
              onChange={() => onPatch({ git_ref_type: "tag" })}
            />
            Tag
          </label>
        </div>
      </div>
      <SelectOrManualField
        label={refType === "tag" ? "Tag" : "分支"}
        value={row.branch}
        options={
          refType === "branch" ? refOptions.filter((name) => !tagNames.includes(name)) : refOptions
        }
        loading={loadingRefs}
        hint={refHint}
        placeholder={refType === "tag" ? "例如 v1.0.1" : "例如 main"}
        emptyLabel={loadingRefs ? "加载中…" : `请选择${refType === "tag" ? "Tag" : "分支"}`}
        onChange={(value) => onPatch({ branch: value })}
      />
      <SelectOrManualField
        label="Dockerfile"
        value={row.dockerfile_path}
        options={dockerfiles}
        pinValue
        loading={loadingDocker}
        hint={dockerHint}
        placeholder="./Dockerfile"
        emptyLabel={loadingDocker ? "加载中…" : "请选择 Dockerfile"}
        onChange={(value) => onPatch({ dockerfile_path: value })}
      />
    </>
  );
}
