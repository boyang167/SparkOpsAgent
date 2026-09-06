"use client";

import { useEffect, useMemo, useState } from "react";
import type { GitRefVM } from "@sparkops/contracts/workspace";
import { BffError } from "@sparkops/contracts";
import { workspace } from "@sparkops/web-sdk";

const GIT_REF_UNSELECTED = "__unset__";

function gitRefPlaceholder(refType: "branch" | "tag") {
  if (refType === "tag") return "请输入标签名称，如：v1.0.0";
  return "请输入分支名称，如：main, develop, feature/xxx";
}

function gitRefKindLabel(refType: "branch" | "tag") {
  return refType === "tag" ? "标签" : "分支";
}

function formatGitRefOption(item: GitRefVM) {
  if (item.title) {
    const short = item.name.length > 8 ? item.name.slice(0, 8) : item.name;
    const title = item.title.length > 60 ? `${item.title.slice(0, 60)}…` : item.title;
    return `${short} - ${title}`;
  }
  return item.name;
}

interface GitRefValueFieldProps {
  workspaceId: string;
  gitUrl?: string;
  secretId?: string;
  gitServiceType?: string;
  refType: "branch" | "tag";
  value: string;
  onChange: (value: string) => void;
  disabled?: boolean;
}

export function GitRefValueField({
  workspaceId,
  gitUrl,
  secretId,
  gitServiceType,
  refType,
  value,
  onChange,
  disabled = false,
}: GitRefValueFieldProps) {
  const [mode, setMode] = useState<"select" | "manual">("select");
  const [remoteRefs, setRemoteRefs] = useState<GitRefVM[]>([]);
  const [loading, setLoading] = useState(false);
  const [loadError, setLoadError] = useState<string | null>(null);

  useEffect(() => {
    setMode("select");
  }, [refType, gitUrl, secretId]);

  useEffect(() => {
    if (mode !== "select" || !workspaceId) {
      setRemoteRefs([]);
      setLoadError(null);
      return;
    }
    const trimmedUrl = gitUrl?.trim();
    const trimmedSecret = secretId?.trim();
    if (!trimmedUrl || !trimmedSecret || gitServiceType !== "GitLab") {
      setRemoteRefs([]);
      return;
    }

    let cancelled = false;
    const timer = window.setTimeout(() => {
      setLoading(true);
      setLoadError(null);
      void workspace
        .listGitRefs(workspaceId, {
          git_url: trimmedUrl,
          secret_id: trimmedSecret,
          ref_type: refType,
        })
        .then((items) => {
          if (cancelled) return;
          setRemoteRefs(items);
        })
        .catch((err: unknown) => {
          if (cancelled) return;
          setRemoteRefs([]);
          const message =
            err instanceof BffError
              ? err.message
              : err instanceof Error
                ? err.message
                : "加载 Git 引用列表失败";
          setLoadError(message);
        })
        .finally(() => {
          if (!cancelled) setLoading(false);
        });
    }, 400);

    return () => {
      cancelled = true;
      window.clearTimeout(timer);
    };
  }, [workspaceId, mode, gitUrl, secretId, refType, gitServiceType]);

  const refOptions = useMemo(() => {
    const current = value.trim();
    if (current && !remoteRefs.some((item) => item.name === current)) {
      return [{ name: current }, ...remoteRefs];
    }
    return remoteRefs;
  }, [value, remoteRefs]);

  const selectedValue = value.trim() ? value : GIT_REF_UNSELECTED;
  const refKind = gitRefKindLabel(refType);

  return (
    <label className="stack-sm">
      <div className="flex items-center justify-between gap-2">
        <span>Git 引用值</span>
        {!disabled ? (
          <button
            type="button"
            className="project-manual-link"
            onClick={() => setMode((current) => (current === "select" ? "manual" : "select"))}
          >
            {mode === "select" ? "手动输入" : "下拉选择"}
          </button>
        ) : null}
      </div>

      {mode === "select" ? (
        <div className="project-git-ref-select">
          <select
            className="input"
            value={selectedValue}
            disabled={disabled || loading}
            required
            onChange={(event) => {
              const next = event.target.value;
              onChange(next === GIT_REF_UNSELECTED ? "" : next);
            }}
          >
            <option value={GIT_REF_UNSELECTED} disabled>
              {loading
                ? `正在加载${refKind}…`
                : refOptions.length
                  ? `请选择${refKind}`
                  : `暂无可用${refKind}`}
            </option>
            {refOptions.map((item) => (
              <option key={item.name} value={item.name}>
                {formatGitRefOption(item)}
              </option>
            ))}
          </select>
          {loading ? (
            <p className="project-form-hint">正在从 Git 仓库加载{refKind}列表…</p>
          ) : null}
          {!loading && loadError ? <p className="project-form-hint">{loadError}</p> : null}
          {!loading && !loadError && !secretId?.trim() ? (
            <p className="project-form-hint">项目未配置 Git 认证秘钥，请改用手动输入</p>
          ) : null}
          {!loading && !loadError && secretId?.trim() && gitServiceType !== "GitLab" ? (
            <p className="project-form-hint">当前仅 GitLab 支持从仓库加载列表，请改用手动输入</p>
          ) : null}
        </div>
      ) : (
        <input
          className="input"
          placeholder={gitRefPlaceholder(refType)}
          value={value}
          disabled={disabled}
          required
          onChange={(event) => onChange(event.target.value)}
        />
      )}
    </label>
  );
}
