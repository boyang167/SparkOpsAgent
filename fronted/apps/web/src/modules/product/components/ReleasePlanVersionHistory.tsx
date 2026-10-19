"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import { product } from "@sparkops/web-sdk";
import { BffError } from "@sparkops/contracts";
import type {
  ReleasePlanVersionDiffVM,
  ReleasePlanVersionSummaryVM,
} from "@sparkops/contracts/product";
import { InlineAlert } from "@/components/ui/InlineAlert";
import { ReleasePlanVersionChangeDetail } from "./ReleasePlanVersionChangeDetail";
import { diffChangeSummary, releasePlanVersionVerbLabel } from "./release-plan-version-utils";

interface ReleasePlanVersionHistoryProps {
  workspaceId: string;
  productId: string;
  planId: string;
  currentVersion?: number;
  refreshKey?: number;
  active?: boolean;
  memberName: (memberId?: string | null) => string;
}

export function ReleasePlanVersionHistory({
  workspaceId,
  productId,
  planId,
  currentVersion,
  refreshKey,
  active = true,
  memberName,
}: ReleasePlanVersionHistoryProps) {
  const [versions, setVersions] = useState<ReleasePlanVersionSummaryVM[]>([]);
  const [hasMore, setHasMore] = useState(false);
  const [selectedVersion, setSelectedVersion] = useState<number | null>(null);
  const [compareVersion, setCompareVersion] = useState<number | "auto">("auto");
  const [diff, setDiff] = useState<ReleasePlanVersionDiffVM | null>(null);
  const [loading, setLoading] = useState(true);
  const [diffLoading, setDiffLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const loadVersions = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const page = await product.listReleasePlanVersions(workspaceId, productId, planId);
      setVersions(page.items);
      setHasMore(page.has_more);
      setSelectedVersion((prev) => {
        if (prev && page.items.some((item) => item.version === prev)) return prev;
        return page.items[0]?.version ?? null;
      });
    } catch (err) {
      setError(err instanceof BffError ? err.message : "加载版本历史失败");
      setVersions([]);
      setHasMore(false);
      setSelectedVersion(null);
    } finally {
      setLoading(false);
    }
  }, [workspaceId, productId, planId]);

  useEffect(() => {
    if (!active) return;
    void loadVersions();
  }, [loadVersions, refreshKey, active]);

  async function loadMoreVersions() {
    try {
      const page = await product.listReleasePlanVersions(workspaceId, productId, planId, versions.length);
      setVersions((prev) => [...prev, ...page.items]);
      setHasMore(page.has_more);
    } catch (err) {
      setError(err instanceof BffError ? err.message : "加载更多版本失败");
    }
  }

  const selectedSummary = useMemo(
    () => versions.find((item) => item.version === selectedVersion) ?? null,
    [versions, selectedVersion],
  );

  const compareOptions = useMemo(() => {
    if (!selectedVersion) return [];
    return versions.filter((item) => item.version < selectedVersion);
  }, [versions, selectedVersion]);

  useEffect(() => {
    if (!selectedVersion) {
      setDiff(null);
      return;
    }
    let cancelled = false;
    setDiffLoading(true);
    const compare =
      compareVersion === "auto"
        ? undefined
        : compareVersion;
    void product
      .getReleasePlanVersionDiff(workspaceId, productId, planId, selectedVersion, compare)
      .then((result) => {
        if (!cancelled) setDiff(result);
      })
      .catch((err) => {
        if (!cancelled) {
          setError(err instanceof BffError ? err.message : "加载版本对比失败");
          setDiff(null);
        }
      })
      .finally(() => {
        if (!cancelled) setDiffLoading(false);
      });
    return () => {
      cancelled = true;
    };
  }, [workspaceId, productId, planId, selectedVersion, compareVersion]);

  useEffect(() => {
    setCompareVersion("auto");
  }, [selectedVersion]);

  if (loading) {
    return <p className="release-plan-version-empty">加载版本历史中…</p>;
  }

  if (versions.length === 0) {
    return (
      <p className="release-plan-version-empty">
        暂无版本记录。编辑发布计划后会自动生成版本快照。
      </p>
    );
  }

  return (
    <div className="release-plan-version-panel">
      {error ? <InlineAlert tone="error">{error}</InlineAlert> : null}

      <div className="release-plan-version-layout">
        <aside className="release-plan-version-list-wrap">
          <div className="release-plan-version-list-head">
            <span className="release-plan-version-list-title">版本列表</span>
            {currentVersion ? (
              <span className="text-xs text-muted-foreground">当前 v{currentVersion}</span>
            ) : null}
          </div>
          <ul className="release-plan-version-list">
            {versions.map((item) => {
              const active = item.version === selectedVersion;
              return (
                <li key={item.version}>
                  <button
                    type="button"
                    className={`release-plan-version-item${active ? " is-active" : ""}`}
                    onClick={() => setSelectedVersion(item.version)}
                  >
                    <span className="release-plan-version-item-main">
                      <span className="release-plan-version-item-no">v{item.version}</span>
                      <span className="release-plan-version-item-verb">
                        {releasePlanVersionVerbLabel(item.verb)}
                      </span>
                    </span>
                    <span className="release-plan-version-item-meta">
                      <span>{memberName(item.actor_id)}</span>
                      <time dateTime={item.created_at}>
                        {new Date(item.created_at).toLocaleString("zh-CN")}
                      </time>
                    </span>
                  </button>
                </li>
              );
            })}
          </ul>
          {hasMore ? (
            <button type="button" className="btn btn-secondary text-xs" onClick={() => void loadMoreVersions()}>
              加载更多
            </button>
          ) : null}
        </aside>

        <section className="release-plan-version-diff-wrap">
          {selectedSummary ? (
            <header className="release-plan-version-diff-head">
              <div className="stack-xs">
                <h3 className="release-plan-version-diff-title">
                  v{selectedSummary.version} · {releasePlanVersionVerbLabel(selectedSummary.verb)}
                </h3>
                <p className="text-xs text-muted-foreground">
                  {memberName(selectedSummary.actor_id)} ·{" "}
                  {new Date(selectedSummary.created_at).toLocaleString("zh-CN")}
                </p>
              </div>
              {compareOptions.length > 0 ? (
                <label className="release-plan-version-compare">
                  <span className="text-xs text-muted-foreground">对比基准</span>
                  <select
                    className="input text-xs"
                    value={compareVersion === "auto" ? "auto" : String(compareVersion)}
                    onChange={(e) => {
                      const value = e.target.value;
                      setCompareVersion(value === "auto" ? "auto" : Number(value));
                    }}
                  >
                    <option value="auto">
                      上一版本
                      {selectedSummary.previous_version > 0
                        ? ` (v${selectedSummary.previous_version})`
                        : ""}
                    </option>
                    {compareOptions.map((item) => (
                      <option key={item.version} value={item.version}>
                        v{item.version} · {releasePlanVersionVerbLabel(item.verb)}
                      </option>
                    ))}
                  </select>
                </label>
              ) : null}
            </header>
          ) : null}

          {diffLoading ? (
            <p className="release-plan-version-empty">加载变更详情…</p>
          ) : !diff || diff.groups.length === 0 ? (
            <p className="release-plan-version-empty">该版本相对基准无可见变更</p>
          ) : (
            <div className="release-plan-version-diff-groups stack-sm">
              {diff.groups.map((group) => (
                <section key={group.group_key} className="release-plan-version-diff-group">
                  <h4 className="release-plan-version-diff-group-title">{group.group_name}</h4>
                  <ul className="release-plan-version-diff-changes">
                    {group.changes.map((change, index) => {
                      const summary = diffChangeSummary(change);
                      return (
                        <li key={`${group.group_key}-${index}`} className="release-plan-version-diff-change">
                          <span className="release-plan-version-diff-change-label">{change.label}</span>
                          {summary ? (
                            <span className="release-plan-version-diff-change-summary">{summary}</span>
                          ) : null}
                          {change.change_type === "order_changed" ? (
                            <div className="release-plan-version-order-diff">
                              <div>
                                <span className="text-xs text-muted-foreground">调整前</span>
                                <p>{change.before_order?.map((item) => item.name).join(" → ") || "—"}</p>
                              </div>
                              <div>
                                <span className="text-xs text-muted-foreground">调整后</span>
                                <p>{change.after_order?.map((item) => item.name).join(" → ") || "—"}</p>
                              </div>
                            </div>
                          ) : null}
                          <ReleasePlanVersionChangeDetail change={change} memberName={memberName} />
                        </li>
                      );
                    })}
                  </ul>
                </section>
              ))}
            </div>
          )}
        </section>
      </div>
    </div>
  );
}
