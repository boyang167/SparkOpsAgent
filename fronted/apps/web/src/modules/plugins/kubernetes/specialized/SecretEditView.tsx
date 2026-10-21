"use client";

import { useEffect, useState } from "react";
import { K8sResourceTabs } from "@/components/k8s";
import {
  formStateToSecret,
  secretToFormState,
  secretTypeLabel,
  type DockerRegistryEntry,
  type KeyValueEntry,
  type SecretFormState,
  type SecretObject,
} from "../secret-view";

type SecretEditViewProps = {
  value: SecretObject;
  saving?: boolean;
  onSave: (next: SecretObject) => Promise<void>;
  onClose: () => void;
};

export function SecretEditView({ value, saving = false, onSave, onClose }: SecretEditViewProps) {
  const [tab, setTab] = useState<"data" | "meta">("data");
  const [form, setForm] = useState<SecretFormState>(() => secretToFormState(value));

  useEffect(() => {
    setForm(secretToFormState(value));
    setTab("data");
  }, [value]);

  async function handleSubmit() {
    await onSave(formStateToSecret(value, form));
  }

  return (
    <div className="stack-md">
      <div className="grid gap-3 md:grid-cols-3">
        <label>
          <span className="label">
            名称 <span className="text-danger">*</span>
          </span>
          <input className="input" value={form.metadata.name} disabled />
        </label>
        <label>
          <span className="label">
            命名空间 <span className="text-danger">*</span>
          </span>
          <input className="input" value={form.metadata.namespace} disabled />
        </label>
        <label>
          <span className="label">类型</span>
          <input className="input" value={secretTypeLabel(form.type)} disabled />
        </label>
      </div>

      <K8sResourceTabs
        tabs={["data", "meta"] as const}
        active={tab}
        ariaLabel="Secret 编辑页签"
        label={(id) => (id === "data" ? "数据" : "标签/注解")}
        onChange={setTab}
      />

      {tab === "data" ? (
        <>
          {form.type === "Opaque" ? (
            <KeyValueSection
              title="数据"
              entries={form.opaqueData}
              valueMultiline
              onChange={(opaqueData) => setForm((current) => ({ ...current, opaqueData }))}
            />
          ) : null}
          {form.type === "kubernetes.io/dockerconfigjson" ? (
            <DockerRegistrySection
              entries={form.dockerEntries}
              onChange={(dockerEntries) => setForm((current) => ({ ...current, dockerEntries }))}
            />
          ) : null}
          {form.type === "kubernetes.io/tls" ? (
            <TLSSection
              cert={form.tlsCert}
              keyValue={form.tlsKey}
              onChange={(tlsCert, tlsKey) => setForm((current) => ({ ...current, tlsCert, tlsKey }))}
            />
          ) : null}
        </>
      ) : (
        <div className="stack-md">
          <KeyValueSection
            title="标签"
            entries={form.metadata.labels}
            onChange={(labels) => setForm((current) => ({ ...current, metadata: { ...current.metadata, labels } }))}
          />
          <KeyValueSection
            title="注解"
            entries={form.metadata.annotations}
            onChange={(annotations) => setForm((current) => ({ ...current, metadata: { ...current.metadata, annotations } }))}
          />
        </div>
      )}

      <div className="flex justify-end gap-2">
        <button type="button" className="btn-secondary" disabled={saving} onClick={onClose}>
          取消
        </button>
        <button type="button" className="btn" disabled={saving} onClick={() => void handleSubmit()}>
          {saving ? "保存中…" : "确定"}
        </button>
      </div>
    </div>
  );
}

function DockerRegistrySection({
  entries,
  onChange,
}: {
  entries: DockerRegistryEntry[];
  onChange: (entries: DockerRegistryEntry[]) => void;
}) {
  return (
    <section className="card stack-sm p-4">
      <h3 className="text-sm font-semibold">私有镜像仓库登录密钥</h3>
      <div className="table-wrap">
        <table className="data-table text-sm">
          <thead>
            <tr>
              <th>仓库地址</th>
              <th>用户名</th>
              <th>密码</th>
              <th />
            </tr>
          </thead>
          <tbody>
            {entries.map((entry, index) => (
              <tr key={`${index}-${entry.registry}`}>
                <td>
                  <input
                    className="input"
                    value={entry.registry}
                    onChange={(e) => updateEntry(entries, index, { registry: e.target.value }, onChange)}
                  />
                </td>
                <td>
                  <input
                    className="input"
                    value={entry.username}
                    onChange={(e) => updateEntry(entries, index, { username: e.target.value }, onChange)}
                  />
                </td>
                <td>
                  <input
                    className="input"
                    type="password"
                    value={entry.password}
                    onChange={(e) => updateEntry(entries, index, { password: e.target.value }, onChange)}
                  />
                </td>
                <td>
                  <button type="button" className="k8s-action-link is-danger" onClick={() => onChange(entries.filter((_, i) => i !== index))}>
                    删除
                  </button>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
      <button
        type="button"
        className="k8s-action-link"
        onClick={() => onChange([...entries, { registry: "", username: "", password: "" }])}
      >
        + 添加
      </button>
    </section>
  );
}

function TLSSection({
  cert,
  keyValue,
  onChange,
}: {
  cert: string;
  keyValue: string;
  onChange: (cert: string, key: string) => void;
}) {
  return (
    <section className="card stack-sm p-4">
      <div className="grid gap-3 md:grid-cols-2">
        <label>
          <span className="label">Cert</span>
          <textarea className="input min-h-40 font-mono text-xs" value={cert} onChange={(e) => onChange(e.target.value, keyValue)} />
        </label>
        <label>
          <span className="label">Key</span>
          <textarea className="input min-h-40 font-mono text-xs" value={keyValue} onChange={(e) => onChange(cert, e.target.value)} />
        </label>
      </div>
    </section>
  );
}

function KeyValueSection({
  title,
  entries,
  valueMultiline = false,
  onChange,
}: {
  title: string;
  entries: KeyValueEntry[];
  valueMultiline?: boolean;
  onChange: (entries: KeyValueEntry[]) => void;
}) {
  return (
    <section className="card stack-sm p-4">
      <h3 className="text-sm font-semibold">{title}</h3>
      <div className="table-wrap">
        <table className="data-table text-sm">
          <thead>
            <tr>
              <th>键</th>
              <th>值</th>
              <th />
            </tr>
          </thead>
          <tbody>
            {entries.map((entry, index) => (
              <tr key={`${index}-${entry.key}`}>
                <td>
                  <input
                    className="input"
                    value={entry.key}
                    onChange={(e) => updateEntry(entries, index, { key: e.target.value }, onChange)}
                  />
                </td>
                <td>
                  {valueMultiline ? (
                    <textarea
                      className="input min-h-20 font-mono text-xs"
                      value={entry.value}
                      onChange={(e) => updateEntry(entries, index, { value: e.target.value }, onChange)}
                    />
                  ) : (
                    <input
                      className="input"
                      value={entry.value}
                      onChange={(e) => updateEntry(entries, index, { value: e.target.value }, onChange)}
                    />
                  )}
                </td>
                <td>
                  <button type="button" className="k8s-action-link is-danger" onClick={() => onChange(entries.filter((_, i) => i !== index))}>
                    删除
                  </button>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
      <button type="button" className="k8s-action-link" onClick={() => onChange([...entries, { key: "", value: "" }])}>
        + 添加
      </button>
    </section>
  );
}

function updateEntry<T extends object>(
  entries: T[],
  index: number,
  patch: Partial<T>,
  onChange: (entries: T[]) => void,
) {
  onChange(entries.map((entry, i) => (i === index ? { ...entry, ...patch } : entry)));
}
