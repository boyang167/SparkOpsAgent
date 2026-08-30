"use client";

import { useState } from "react";
import { ApiErrorBanner } from "../ApiErrorBanner";
import { errorInfo, k8sFetch, type ApiErrorInfo } from "../api";
import { CATALOG, buildDeploymentManifest, proxyPath, validateDeploymentDraft, type DeploymentDraft } from "../catalog";
import { Field } from "./FormParts";

const deploymentDef = CATALOG.find((item) => item.kind === "Deployment");

export function CreateDeploymentPage({ clusterId, canManage }: { clusterId: string; canManage: boolean }) {
  const [draft, setDraft] = useState<DeploymentDraft>({
    namespace: "default",
    name: "",
    image: "",
    replicas: 1,
    ports: "80",
  });
  const [error, setError] = useState<ApiErrorInfo | string | null>(null);
  const [submitting, setSubmitting] = useState(false);
  const [created, setCreated] = useState<string | null>(null);
  const errors = validateDeploymentDraft(draft);
  const disabled = !clusterId || !canManage || Object.keys(errors).length > 0 || !deploymentDef;

  async function submit() {
    if (!deploymentDef) return;
    const nextErrors = validateDeploymentDraft(draft);
    if (Object.keys(nextErrors).length > 0) {
      setError("请先修正表单校验错误");
      return;
    }
    setSubmitting(true);
    setError(null);
    setCreated(null);
    try {
      await k8sFetch(proxyPath(clusterId, deploymentDef, draft.namespace), {
        method: "POST",
        body: JSON.stringify(buildDeploymentManifest(draft)),
      });
      setCreated(`${draft.namespace}/${draft.name}`);
    } catch (err) {
      setError(errorInfo(err, "创建 Deployment 失败"));
    } finally {
      setSubmitting(false);
    }
  }

  return (
    <div className="stack-md">
      {!canManage ? <div className="alert alert-info">你没有 governance:admin 权限，创建入口只读。</div> : null}
      <div className="grid gap-3 md:grid-cols-2">
        <Field label="命名空间" error={errors.namespace}>
          <input className="input" value={draft.namespace} onChange={(e) => setDraft({ ...draft, namespace: e.target.value })} />
        </Field>
        <Field label="名称" error={errors.name}>
          <input className="input" value={draft.name} onChange={(e) => setDraft({ ...draft, name: e.target.value })} />
        </Field>
        <Field label="镜像" error={errors.image}>
          <input className="input" value={draft.image} placeholder="nginx:1.27" onChange={(e) => setDraft({ ...draft, image: e.target.value })} />
        </Field>
        <Field label="副本数" error={errors.replicas}>
          <input className="input" type="number" min={1} value={draft.replicas} onChange={(e) => setDraft({ ...draft, replicas: Number(e.target.value) })} />
        </Field>
        <Field label="容器端口（逗号分隔）" error={errors.ports}>
          <input className="input" value={draft.ports} onChange={(e) => setDraft({ ...draft, ports: e.target.value })} />
        </Field>
      </div>
      <ApiErrorBanner error={error} />
      {created ? <div className="alert alert-info">已提交创建：{created}</div> : null}
      <div className="flex justify-end">
        <button type="button" className="btn" disabled={disabled || submitting} onClick={() => void submit()}>
          {submitting ? "创建中…" : "创建 Deployment"}
        </button>
      </div>
    </div>
  );
}
