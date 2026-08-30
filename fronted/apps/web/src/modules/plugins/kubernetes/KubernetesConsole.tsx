"use client";

import { useEffect, useMemo, useState } from "react";
import { ApiErrorBanner } from "./ApiErrorBanner";
import { ClusterPicker, type KubernetesCluster } from "./ClusterPicker";
import { ResourceMenuPage } from "./ResourceMenuPage";
import { errorInfo, k8sFetch, type ApiErrorInfo } from "./api";
import { CATALOG, buildDeploymentManifest, proxyPath, validateDeploymentDraft, type DeploymentDraft } from "./catalog";

export type KubernetesConsoleMode =
  | "overview"
  | "clusters"
  | "resource"
  | "create"
  | "records"
  | "diagnosis"
  | "chat"
  | "cloudtty"
  | "velaux";

type KubernetesConsoleProps = {
  mode: KubernetesConsoleMode;
  title: string;
  description?: string;
  menu?: string;
  canManage: boolean;
};

type RecordItem = {
  id: string;
  clusterId: string;
  kind: string;
  startedAt: string;
  endedAt?: string;
  objectPath?: string;
};

type RecordsResponse = {
  items?: RecordItem[];
  total?: number;
};

const deploymentDef = CATALOG.find((item) => item.kind === "Deployment");

export function KubernetesConsole({ mode, title, description, menu, canManage }: KubernetesConsoleProps) {
  const [clusterId, setClusterId] = useState("");
  const [clusters, setClusters] = useState<KubernetesCluster[]>([]);

  const body = useMemo(() => {
    if (mode === "overview") return <Overview />;
    if (mode === "clusters") return <Clusters clusters={clusters} />;
    if (mode === "create") return <CreateDeployment clusterId={clusterId} canManage={canManage} />;
    if (mode === "records") return <Records canManage={canManage} />;
    if (mode === "diagnosis") return <Diagnosis clusterId={clusterId} canManage={canManage} />;
    if (mode === "chat") return <Chat clusterId={clusterId} canManage={canManage} />;
    if (mode === "cloudtty") return <CloudTTY clusterId={clusterId} canManage={canManage} />;
    if (mode === "velaux") return <Velaux clusterId={clusterId} />;
    return (
      <ResourceMenuPage
        title={title}
        description={description}
        menu={menu ?? ""}
        clusterId={clusterId}
        canManage={canManage}
      />
    );
  }, [canManage, clusterId, clusters, description, menu, mode, title]);

  return (
    <div className="page stack-lg">
      <ClusterPicker
        value={clusterId}
        onChange={setClusterId}
        onClustersChange={setClusters}
        canManage={canManage}
        showSettings={mode === "clusters"}
      />
      {mode === "resource" ? body : (
        <section className="card stack-md p-4">
          <div>
            <h1 className="text-xl font-semibold">{title}</h1>
            {description ? <p className="text-sm text-muted mt-1">{description}</p> : null}
          </div>
          {body}
        </section>
      )}
    </div>
  );
}

function Overview() {
  return (
    <div className="grid-cards">
      {[
        ["资源目录", "覆盖核心、apps、batch、networking、Gateway API、RBAC 与常见 CRD。"],
        ["集群选择", "集群来源于当前工作区授权列表，选择持久化在本地。"],
        ["诊断入口", "Chat、CloudTTY、Arthas、Velaux 均通过同源 BFF 路径访问。"],
      ].map(([name, desc]) => (
        <div key={name} className="list-item">
          <div>
            <div className="font-medium">{name}</div>
            <div className="text-sm text-muted mt-1">{desc}</div>
          </div>
        </div>
      ))}
    </div>
  );
}

function Clusters({ clusters }: { clusters: KubernetesCluster[] }) {
  return (
    <div className="table-wrap">
      <table className="data-table">
        <thead>
          <tr>
            <th>名称</th>
            <th>ID</th>
            <th>Server</th>
            <th>Prometheus</th>
            <th>Velaux</th>
            <th>LLM</th>
          </tr>
        </thead>
        <tbody>
          {clusters.map((item) => (
            <tr key={item.id}>
              <td>{item.name || "-"}</td>
              <td>{item.id}</td>
              <td>{item.server || "-"}</td>
              <td>{item.prometheus_url || "-"}</td>
              <td>{item.velaux_url || "-"}</td>
              <td>{item.llm_model || "-"}</td>
            </tr>
          ))}
          {clusters.length === 0 ? (
            <tr>
              <td colSpan={6}>
                <div className="empty-state">暂无集群</div>
              </td>
            </tr>
          ) : null}
        </tbody>
      </table>
    </div>
  );
}

function CreateDeployment({ clusterId, canManage }: { clusterId: string; canManage: boolean }) {
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
      const manifest = buildDeploymentManifest(draft);
      await k8sFetch(proxyPath(clusterId, deploymentDef, draft.namespace), {
        method: "POST",
        body: JSON.stringify(manifest),
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

function Records({ canManage }: { canManage: boolean }) {
  const [items, setItems] = useState<RecordItem[]>([]);
  const [detail, setDetail] = useState("");
  const [error, setError] = useState<ApiErrorInfo | null>(null);

  async function load() {
    setError(null);
    try {
      const data = await k8sFetch<RecordsResponse>("/api/bff/v1/plugins/kubernetes/records?page=1&pageSize=20");
      setItems(data.items ?? []);
    } catch (err) {
      setError(errorInfo(err, "加载终端审计失败"));
    }
  }

  useEffect(() => {
    void load();
  }, []);

  async function open(id: string) {
    setError(null);
    try {
      setDetail(await k8sFetch<string>(`/api/bff/v1/plugins/kubernetes/records/${encodeURIComponent(id)}`));
    } catch (err) {
      setError(errorInfo(err, "加载记录详情失败"));
    }
  }

  async function remove(id: string) {
    if (!window.confirm("确认删除该终端审计记录？")) return;
    try {
      await k8sFetch<void>(`/api/bff/v1/plugins/kubernetes/records/${encodeURIComponent(id)}`, { method: "DELETE" });
      await load();
    } catch (err) {
      setError(errorInfo(err, "删除记录失败"));
    }
  }

  return (
    <div className="stack-md">
      <div className="flex justify-end">
        <button type="button" className="btn-secondary" onClick={() => void load()}>
          刷新
        </button>
      </div>
      <ApiErrorBanner error={error} />
      <div className="table-wrap">
        <table className="data-table">
          <thead>
            <tr>
              <th>ID</th>
              <th>集群</th>
              <th>类型</th>
              <th>开始时间</th>
              <th>对象</th>
              <th>操作</th>
            </tr>
          </thead>
          <tbody>
            {items.map((item) => (
              <tr key={item.id}>
                <td>{item.id}</td>
                <td>{item.clusterId}</td>
                <td>{item.kind}</td>
                <td>{item.startedAt}</td>
                <td>{item.objectPath || "-"}</td>
                <td>
                  <div className="flex gap-2">
                    <button type="button" className="btn-secondary text-xs" onClick={() => void open(item.id)}>
                      详情
                    </button>
                    {canManage ? (
                      <button type="button" className="btn-secondary text-xs" onClick={() => void remove(item.id)}>
                        删除
                      </button>
                    ) : null}
                  </div>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
      {detail ? <pre className="input input-mono min-h-48 whitespace-pre-wrap text-xs">{detail}</pre> : null}
    </div>
  );
}

function Diagnosis({ clusterId, canManage }: { clusterId: string; canManage: boolean }) {
  const [form, setForm] = useState({ namespace: "default", pod: "", container: "", pid: "1" });
  const [sessionId, setSessionId] = useState("");
  const [lines, setLines] = useState<string[]>([]);
  const [error, setError] = useState<ApiErrorInfo | null>(null);

  async function start() {
    setError(null);
    setLines([]);
    try {
      const data = await k8sFetch<{ id: string; status: string }>(
        `/api/bff/v1/plugins/kubernetes/clusters/${encodeURIComponent(clusterId)}/diagnosis`,
        { method: "POST", body: JSON.stringify({ ...form, pid: Number(form.pid) }) },
      );
      setSessionId(data.id);
    } catch (err) {
      setError(errorInfo(err, "启动诊断失败"));
    }
  }

  useEffect(() => {
    if (!sessionId) return;
    const source = new EventSource(`/api/bff/v1/plugins/kubernetes/diagnosis/${encodeURIComponent(sessionId)}/ws`);
    source.addEventListener("output", (event) => setLines((prev) => [...prev, event.data]));
    source.addEventListener("error", (event) => setLines((prev) => [...prev, "诊断流错误或已结束"]));
    source.addEventListener("exit", () => source.close());
    return () => source.close();
  }, [sessionId]);

  return (
    <EndpointForm disabled={!clusterId || !canManage} submitLabel="启动 Arthas 诊断" onSubmit={start}>
      <TargetFields form={form} setForm={setForm} />
      <Field label="PID">
        <input className="input" value={form.pid} onChange={(e) => setForm({ ...form, pid: e.target.value })} />
      </Field>
      <ApiErrorBanner error={error} />
      <pre className="input input-mono min-h-48 whitespace-pre-wrap text-xs">{lines.join("\n")}</pre>
    </EndpointForm>
  );
}

function Chat({ clusterId, canManage }: { clusterId: string; canManage: boolean }) {
  const [prompt, setPrompt] = useState("");
  const [output, setOutput] = useState("");
  const [error, setError] = useState<ApiErrorInfo | null>(null);

  async function send() {
    setError(null);
    setOutput("");
    try {
      const res = await fetch(`/api/bff/v1/plugins/kubernetes/clusters/${encodeURIComponent(clusterId)}/chat/query`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ prompt }),
      });
      if (!res.ok) throw new Error((await res.text()) || "Chat 请求失败");
      const reader = res.body?.getReader();
      const decoder = new TextDecoder();
      while (reader) {
        const { value, done } = await reader.read();
        if (done) break;
        setOutput((prev) => prev + decoder.decode(value, { stream: true }));
      }
    } catch (err) {
      setError(errorInfo(err, "Chat 请求失败"));
    }
  }

  return (
    <EndpointForm disabled={!clusterId || !canManage || !prompt.trim()} submitLabel="发送" onSubmit={send}>
      <label>
        <span className="label">问题</span>
        <textarea className="input min-h-28" value={prompt} onChange={(e) => setPrompt(e.target.value)} />
      </label>
      <ApiErrorBanner error={error} />
      <pre className="input input-mono min-h-48 whitespace-pre-wrap text-xs">{output}</pre>
    </EndpointForm>
  );
}

function CloudTTY({ clusterId, canManage }: { clusterId: string; canManage: boolean }) {
  const [form, setForm] = useState({ namespace: "default", pod: "", container: "" });
  const [result, setResult] = useState("");
  const [error, setError] = useState<ApiErrorInfo | null>(null);

  async function start() {
    setError(null);
    setResult("");
    try {
      const data = await k8sFetch(`/api/bff/v1/plugins/kubernetes/clusters/${encodeURIComponent(clusterId)}/cloudtty`, {
        method: "POST",
        body: JSON.stringify(form),
      });
      setResult(JSON.stringify(data, null, 2));
    } catch (err) {
      setError(errorInfo(err, "创建 CloudTTY 失败"));
    }
  }

  return (
    <EndpointForm disabled={!clusterId || !canManage} submitLabel="创建 CloudTTY 会话" onSubmit={start}>
      <TargetFields form={form} setForm={setForm} />
      <ApiErrorBanner error={error} />
      <pre className="input input-mono min-h-40 whitespace-pre-wrap text-xs">{result}</pre>
    </EndpointForm>
  );
}

function Velaux({ clusterId }: { clusterId: string }) {
  if (!clusterId) return <div className="empty-state">请先选择集群。</div>;
  const src = `/api/bff/v1/plugins/kubernetes/clusters/${encodeURIComponent(clusterId)}/velaux/`;
  return (
    <div className="stack-sm">
      <div className="alert alert-info">Velaux 通过同源 BFF proxy 打开，不直接暴露后端配置 URL 或密钥。</div>
      <iframe title="Velaux" src={src} className="w-full min-h-[640px] rounded-md border" />
    </div>
  );
}

function EndpointForm({
  children,
  disabled,
  submitLabel,
  onSubmit,
}: {
  children: React.ReactNode;
  disabled: boolean;
  submitLabel: string;
  onSubmit: () => Promise<void>;
}) {
  return (
    <form
      className="stack-md"
      onSubmit={(e) => {
        e.preventDefault();
        void onSubmit();
      }}
    >
      {disabled ? <div className="alert alert-info">需要选择集群并拥有 governance:admin 权限。</div> : null}
      <div className="grid gap-3 md:grid-cols-3">{children}</div>
      <div className="flex justify-end">
        <button type="submit" className="btn" disabled={disabled}>
          {submitLabel}
        </button>
      </div>
    </form>
  );
}

function TargetFields<T extends { namespace: string; pod: string; container: string }>({
  form,
  setForm,
}: {
  form: T;
  setForm: (next: T) => void;
}) {
  return (
    <>
      <Field label="命名空间">
        <input className="input" value={form.namespace} onChange={(e) => setForm({ ...form, namespace: e.target.value })} />
      </Field>
      <Field label="Pod">
        <input className="input" value={form.pod} onChange={(e) => setForm({ ...form, pod: e.target.value })} />
      </Field>
      <Field label="容器">
        <input className="input" value={form.container} onChange={(e) => setForm({ ...form, container: e.target.value })} />
      </Field>
    </>
  );
}

function Field({ label, error, children }: { label: string; error?: string; children: React.ReactNode }) {
  return (
    <label>
      <span className="label">{label}</span>
      {children}
      {error ? <span className="error mt-1 block">{error}</span> : null}
    </label>
  );
}
