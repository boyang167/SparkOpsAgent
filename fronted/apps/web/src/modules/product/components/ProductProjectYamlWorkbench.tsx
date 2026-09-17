"use client";

import { useEffect, useMemo, useRef, useState } from "react";
import type { EnvironmentVM } from "@sparkops/contracts/product";
import { InlineAlert } from "@/components/ui/InlineAlert";
import { Modal } from "@/components/ui/Modal";
import { applyNamespaceToYaml, applyResourceToCluster, extractLabels, type ProductK8sResource } from "./product-k8s-yaml";

type InspectorTab = "info" | "changes" | "resources" | "events";

interface ProductProjectYamlWorkbenchProps {
  resource: ProductK8sResource | null;
  yaml: string;
  onYamlChange: (value: string) => void;
  environments: EnvironmentVM[];
  saving: boolean;
  dirty: boolean;
  error?: string | null;
  hint?: string | null;
  onSave: () => void;
  onUnbind?: () => void;
}

const INSPECTOR_TABS: Array<{ id: InspectorTab; label: string }> = [
  { id: "info", label: "基本信息" },
  { id: "changes", label: "变更" },
  { id: "resources", label: "资源" },
  { id: "events", label: "事件" },
];

export function ProductProjectYamlWorkbench({
  resource,
  yaml,
  onYamlChange,
  environments,
  saving,
  dirty,
  error,
  hint,
  onSave,
  onUnbind,
}: ProductProjectYamlWorkbenchProps) {
  const gutterRef = useRef<HTMLDivElement>(null);
  const [inspectorTab, setInspectorTab] = useState<InspectorTab>("info");
  const [joinOpen, setJoinOpen] = useState(false);
  const [envId, setEnvId] = useState("");
  const [joinError, setJoinError] = useState<string | null>(null);
  const [joining, setJoining] = useState(false);
  const lines = useMemo(() => (yaml.length > 0 ? yaml.split("\n") : [""]), [yaml]);
  const labels = useMemo(() => extractLabels(yaml), [yaml]);

  useEffect(() => {
    if (environments.length > 0 && !environments.some((item) => item.id === envId)) {
      setEnvId(environments[0].id);
    }
  }, [environments, envId]);

  async function onJoinEnvironment() {
    if (!resource) return;
    const env = environments.find((item) => item.id === envId);
    if (!env) {
      setJoinError("请选择要加入的环境");
      return;
    }
    setJoining(true);
    setJoinError(null);
    try {
      const payload: ProductK8sResource = {
        ...resource,
        yaml: applyNamespaceToYaml(yaml, env.namespace) || yaml,
      };
      await applyResourceToCluster(payload, env.cluster_id, env.namespace);
      setJoinOpen(false);
    } catch (err) {
      setJoinError(err instanceof Error ? err.message : "加入环境失败");
    } finally {
      setJoining(false);
    }
  }

  if (!resource) {
    return (
      <div className="product-yaml-workbench">
        <div className="product-yaml-empty">
          <strong>选择左侧资源</strong>
          <span>查看并编辑 Kubernetes YAML，保存后可加入产品环境。</span>
        </div>
      </div>
    );
  }

  const sourceLabel = resource.source === "cluster" ? "集群" : "模板";

  return (
    <div className="product-yaml-workbench">
      {error ? <InlineAlert tone="error">{error}</InlineAlert> : null}
      {hint ? <p className="product-yaml-hint">{hint}</p> : null}

      <div className="product-yaml-editor-head">
        <div className="product-yaml-editor-title">
          <span className="product-yaml-kind">{resource.kind}</span>
          <span className="product-yaml-res-name">{resource.name}</span>
          {dirty ? <span className="badge badge-warning">未保存</span> : null}
        </div>
        <span className={`product-yaml-source${resource.source === "cluster" ? " is-cluster" : ""}`}>
          {sourceLabel}
        </span>
      </div>

      <div className="product-yaml-body">
        <div className="product-yaml-editor">
          <div
            className="k8s-yaml-editor product-yaml-code"
            data-theme="light"
          >
            <div className="k8s-yaml-gutter" aria-hidden ref={gutterRef}>
              {lines.map((_, index) => (
                <div key={index}>{index + 1}</div>
              ))}
            </div>
            <textarea
              className="k8s-yaml-textarea"
              value={yaml}
              spellCheck={false}
              aria-label={`${resource.kind}/${resource.name} YAML`}
              onChange={(e) => onYamlChange(e.target.value)}
              onScroll={(e) => {
                if (gutterRef.current) gutterRef.current.scrollTop = e.currentTarget.scrollTop;
              }}
            />
          </div>
        </div>

        {inspectorTab === "info" ? (
          <aside className="product-yaml-inspector" aria-label="基本信息">
            <h3>基本信息</h3>
            <dl className="product-yaml-meta">
              <div>
                <dt>类型</dt>
                <dd>{resource.kind}</dd>
              </div>
              <div>
                <dt>名称</dt>
                <dd>{resource.name}</dd>
              </div>
              {resource.namespace ? (
                <div>
                  <dt>Namespace</dt>
                  <dd className="font-mono">{resource.namespace}</dd>
                </div>
              ) : null}
              <div>
                <dt>来源</dt>
                <dd>{sourceLabel}</dd>
              </div>
            </dl>
            <div className="product-yaml-inspector-row">标签</div>
            {labels.length === 0 ? (
              <p className="product-yaml-inspector-empty">当前 YAML 未声明标签</p>
            ) : (
              <>
                <div className="product-yaml-kv-head">
                  <span>键</span>
                  <span>值</span>
                </div>
                <ul className="product-yaml-kv-list">
                  {labels.map((item) => (
                    <li key={item.key}>
                      <span>{item.key}</span>
                      <span>{item.value}</span>
                    </li>
                  ))}
                </ul>
              </>
            )}
          </aside>
        ) : inspectorTab === "changes" ? (
          <aside className="product-yaml-inspector" aria-label="变更">
            <h3>变更</h3>
            <p className="product-yaml-inspector-empty">
              {dirty ? "YAML 已修改，保存后才会写回模板或集群。" : "当前内容与上次保存一致。"}
            </p>
          </aside>
        ) : (
          <aside className="product-yaml-inspector" aria-label={INSPECTOR_TABS.find((item) => item.id === inspectorTab)?.label}>
            <h3>{INSPECTOR_TABS.find((item) => item.id === inspectorTab)?.label}</h3>
            <p className="product-yaml-inspector-empty">该面板即将支持，先在 YAML 中查看完整定义。</p>
          </aside>
        )}

        <nav className="product-yaml-rail" aria-label="资源详情">
          {INSPECTOR_TABS.map((item) => (
            <button
              key={item.id}
              type="button"
              className={inspectorTab === item.id ? "is-active" : ""}
              onClick={() => setInspectorTab(item.id)}
            >
              {item.label}
            </button>
          ))}
        </nav>
      </div>

      <div className="product-yaml-footer">
        <div className="product-yaml-footer-left">
          {onUnbind ? (
            <button type="button" className="btn-ghost text-sm product-yaml-unbind" onClick={onUnbind}>
              解绑项目
            </button>
          ) : null}
        </div>
        <div className="product-yaml-footer-right">
          <button type="button" className="btn btn-secondary text-sm" onClick={() => setJoinOpen(true)}>
            加入环境
          </button>
          <button type="button" className="btn btn-primary text-sm" disabled={saving || !dirty} onClick={onSave}>
            {saving ? "保存中…" : "保存"}
          </button>
        </div>
      </div>

      <Modal
        open={joinOpen}
        onClose={() => setJoinOpen(false)}
        title="加入环境"
        description={`将 ${resource.kind.toLowerCase()}/${resource.name} 应用到所选环境的集群命名空间。`}
        footer={
          <>
            <button type="button" className="btn btn-secondary text-sm" onClick={() => setJoinOpen(false)}>
              取消
            </button>
            <button
              type="button"
              className="btn btn-primary text-sm"
              disabled={joining || environments.length === 0}
              onClick={() => void onJoinEnvironment()}
            >
              {joining ? "加入中…" : "确认加入"}
            </button>
          </>
        }
      >
        <div className="stack-sm text-sm">
          {joinError ? <InlineAlert tone="error">{joinError}</InlineAlert> : null}
          {environments.length === 0 ? (
            <p className="text-muted-foreground">当前产品还没有环境，请先在「环境」Tab 创建。</p>
          ) : (
            <label className="stack-xs">
              <span className="text-muted-foreground">目标环境</span>
              <select className="input" value={envId} onChange={(e) => setEnvId(e.target.value)}>
                {environments.map((env) => (
                  <option key={env.id} value={env.id}>
                    {env.name} · {env.namespace}
                  </option>
                ))}
              </select>
            </label>
          )}
        </div>
      </Modal>
    </div>
  );
}
