"use client";

import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { BffError } from "@sparkops/contracts";
import type { ProjectConfigMapVM } from "@sparkops/contracts/cluster";
import type { ProjectVM } from "@sparkops/contracts/workspace";
import { cluster } from "@sparkops/web-sdk";
import { Modal } from "@/components/ui/Modal";
import {
  dataToRows,
  isValidConfigMapName,
  newConfigMapRow,
  parseConfigMapYaml,
  readTextFile,
  rowsToData,
  sanitizeConfigMapKey,
  serializeConfigMapYaml,
  type ConfigMapRow,
} from "./configmap-form";

interface ProjectConfigMapTabProps {
  workspaceId: string;
  project: ProjectVM;
  active: boolean;
}

type EditorMode = "visual" | "yaml";

function formatConfigMapError(err: unknown, fallback: string) {
  if (!(err instanceof BffError)) return fallback;
  if (err.status === 404 || /invalid response|not found/i.test(err.message)) {
    return "ConfigMap 服务未就绪（请确认后端已重启并加载最新接口）";
  }
  if (err.status === 503 || /unavailable/i.test(err.message)) {
    return "后端服务不可用，请稍后重试";
  }
  return err.message || fallback;
}

function ConfigMapKvTable({
  rows,
  onChange,
  onImportFiles,
}: {
  rows: ConfigMapRow[];
  onChange: (rows: ConfigMapRow[]) => void;
  onImportFiles: (files: FileList) => void;
}) {
  const fileRef = useRef<HTMLInputElement>(null);

  function updateRow(id: string, patch: Partial<ConfigMapRow>) {
    onChange(rows.map((row) => (row.id === id ? { ...row, ...patch } : row)));
  }

  return (
    <div className="stack-sm project-configmap-kv-editor">
      <div className="project-configmap-kv-toolbar">
        <span>配置数据</span>
        <div className="flex items-center gap-2">
          <input
            ref={fileRef}
            type="file"
            multiple
            hidden
            onChange={(event) => {
              if (event.target.files?.length) onImportFiles(event.target.files);
              event.target.value = "";
            }}
          />
          <button type="button" className="btn btn-secondary text-sm" onClick={() => fileRef.current?.click()}>
            导入文件
          </button>
          <button
            type="button"
            className="btn btn-secondary text-sm"
            onClick={() => onChange([...rows, newConfigMapRow()])}
          >
            + 添加一行
          </button>
        </div>
      </div>
      <div className="project-configmap-table-wrap is-flush">
        <table className="project-configmap-table project-configmap-editor-table">
          <thead>
            <tr>
              <th style={{ width: 200 }}>Key</th>
              <th>Value</th>
              <th style={{ width: 72 }}>操作</th>
            </tr>
          </thead>
          <tbody>
            {rows.map((row) => (
              <tr key={row.id}>
                <td>
                  <input
                    className="input"
                    value={row.key}
                    placeholder="KEY"
                    onChange={(event) => updateRow(row.id, { key: event.target.value })}
                  />
                </td>
                <td>
                  <textarea
                    className="input"
                    rows={2}
                    value={row.value}
                    placeholder="value"
                    onChange={(event) => updateRow(row.id, { value: event.target.value })}
                  />
                </td>
                <td>
                  <button
                    type="button"
                    className="project-runtime-pod-link is-danger"
                    onClick={() =>
                      onChange(rows.length === 1 ? [newConfigMapRow()] : rows.filter((item) => item.id !== row.id))
                    }
                  >
                    删除
                  </button>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </div>
  );
}

export function ProjectConfigMapTab({ workspaceId, project, active }: ProjectConfigMapTabProps) {
  const [items, setItems] = useState<ProjectConfigMapVM[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [selectedName, setSelectedName] = useState<string | null>(null);
  const [createOpen, setCreateOpen] = useState(false);
  const [createName, setCreateName] = useState(project.name);
  const [createRows, setCreateRows] = useState<ConfigMapRow[]>([newConfigMapRow()]);
  const [createMode, setCreateMode] = useState<EditorMode>("visual");
  const [createYaml, setCreateYaml] = useState("");
  const [detailMode, setDetailMode] = useState<EditorMode>("visual");
  const [detailYaml, setDetailYaml] = useState("");
  const [detailYamlDirty, setDetailYamlDirty] = useState(false);
  const [detailError, setDetailError] = useState<string | null>(null);
  const [entryOpen, setEntryOpen] = useState(false);
  const [editingKey, setEditingKey] = useState<string | null>(null);
  const [entryKey, setEntryKey] = useState("");
  const [entryValue, setEntryValue] = useState("");
  const [saving, setSaving] = useState(false);
  const [formError, setFormError] = useState<string | null>(null);
  const entryFileRef = useRef<HTMLInputElement>(null);

  const selected = useMemo(
    () => items.find((item) => item.name === selectedName) ?? items[0] ?? null,
    [items, selectedName],
  );
  const namespace = project.namespace || workspaceId;

  const load = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const next = await cluster.listConfigMaps(project.id, workspaceId);
      setItems(next);
      setSelectedName((current) => {
        if (current && next.some((item) => item.name === current)) return current;
        return next[0]?.name ?? null;
      });
    } catch (err) {
      setError(formatConfigMapError(err, "ConfigMap 加载失败"));
      setItems([]);
      setSelectedName(null);
    } finally {
      setLoading(false);
    }
  }, [project.id, workspaceId]);

  useEffect(() => {
    if (!active) return;
    void load();
  }, [active, load]);

  useEffect(() => {
    if (!selected) {
      setDetailYaml("");
      setDetailYamlDirty(false);
      setDetailMode("visual");
      setDetailError(null);
      return;
    }
    if (detailMode === "yaml" && detailYamlDirty) return;
    setDetailYaml(serializeConfigMapYaml(selected.name, selected.namespace || namespace, selected.data ?? {}));
    setDetailYamlDirty(false);
    setDetailError(null);
  }, [selected, selected?.name, selected?.namespace, selected?.data, namespace, detailMode, detailYamlDirty]);

  function switchDetailMode(next: EditorMode) {
    if (!selected || next === detailMode) return;
    setDetailError(null);
    if (next === "yaml") {
      setDetailYaml(serializeConfigMapYaml(selected.name, selected.namespace || namespace, selected.data ?? {}));
      setDetailYamlDirty(false);
      setDetailMode("yaml");
      return;
    }
    if (detailYamlDirty) {
      try {
        parseConfigMapYaml(detailYaml);
      } catch (err) {
        setDetailError(err instanceof Error ? err.message : "YAML 无法解析，请修正后再切换");
        return;
      }
      if (!window.confirm("YAML 有未保存修改，切换回可视化将丢弃这些修改，是否继续？")) {
        return;
      }
    }
    setDetailYamlDirty(false);
    setDetailMode("visual");
  }

  async function handleSaveDetailYaml() {
    if (!selected) return;
    let data: Record<string, string>;
    try {
      data = parseConfigMapYaml(detailYaml).data;
    } catch (err) {
      setDetailError(err instanceof Error ? err.message : "YAML 无法解析");
      return;
    }
    setSaving(true);
    setDetailError(null);
    try {
      await cluster.replaceConfigMapData(project.id, workspaceId, selected.name, { data });
      setDetailYamlDirty(false);
      await load();
    } catch (err) {
      setDetailError(formatConfigMapError(err, "保存 YAML 失败"));
    } finally {
      setSaving(false);
    }
  }

  function openCreateModal() {
    const name = project.name;
    setCreateName(name);
    setCreateRows([newConfigMapRow()]);
    setCreateMode("visual");
    setCreateYaml(serializeConfigMapYaml(name, namespace, {}));
    setFormError(null);
    setCreateOpen(true);
  }

  function switchCreateMode(next: EditorMode) {
    setFormError(null);
    if (next === createMode) return;
    if (next === "yaml") {
      const parsed = rowsToData(createRows);
      if (parsed.error) {
        setFormError(parsed.error);
        return;
      }
      setCreateYaml(serializeConfigMapYaml(createName.trim() || project.name, namespace, parsed.data));
      setCreateMode("yaml");
      return;
    }
    try {
      const parsed = parseConfigMapYaml(createYaml);
      if (parsed.name?.trim()) setCreateName(parsed.name.trim());
      setCreateRows(dataToRows(parsed.data));
      setCreateMode("visual");
    } catch (err) {
      setFormError(err instanceof Error ? err.message : "YAML 无法解析，请修正后再切换");
    }
  }

  async function importFilesToRows(files: FileList, target: "create" | "entry") {
    setFormError(null);
    try {
      const imported: ConfigMapRow[] = [];
      for (const file of Array.from(files)) {
        const value = await readTextFile(file);
        imported.push(newConfigMapRow({ key: sanitizeConfigMapKey(file.name), value }));
      }
      if (target === "entry") {
        const first = imported[0];
        if (!first) return;
        if (!editingKey) setEntryKey(first.key);
        setEntryValue(first.value);
        return;
      }
      setCreateRows((current) => {
        const usable = current.filter((row) => row.key.trim() || row.value.trim());
        return [...usable, ...imported];
      });
    } catch (err) {
      setFormError(err instanceof Error ? err.message : "读取文件失败");
    }
  }

  async function handleCreate() {
    const name = createName.trim() || project.name;
    if (!isValidConfigMapName(name)) {
      setFormError("名称需为小写字母、数字和短横线，例如 test-repo");
      return;
    }
    let data: Record<string, string> = {};
    if (createMode === "yaml") {
      try {
        const parsed = parseConfigMapYaml(createYaml);
        data = parsed.data;
        if (parsed.name?.trim()) {
          /* keep current name field as source of truth */
        }
      } catch (err) {
        setFormError(err instanceof Error ? err.message : "YAML 无法解析");
        return;
      }
    } else {
      const parsed = rowsToData(createRows);
      if (parsed.error) {
        setFormError(parsed.error);
        return;
      }
      data = parsed.data;
    }
    setSaving(true);
    setFormError(null);
    try {
      await cluster.createConfigMap(project.id, workspaceId, { name, data });
      setCreateOpen(false);
      await load();
      setSelectedName(name);
    } catch (err) {
      setFormError(formatConfigMapError(err, "创建 ConfigMap 失败"));
    } finally {
      setSaving(false);
    }
  }

  async function handleSaveEntry() {
    if (!selected) return;
    const key = entryKey.trim();
    if (!key) {
      setFormError("Key 不能为空");
      return;
    }
    setSaving(true);
    setFormError(null);
    try {
      await cluster.upsertConfigMapEntry(project.id, workspaceId, selected.name, {
        key,
        value: entryValue,
      });
      setEntryOpen(false);
      await load();
    } catch (err) {
      setFormError(formatConfigMapError(err, "保存配置数据失败"));
    } finally {
      setSaving(false);
    }
  }

  async function handleDeleteEntry(key: string) {
    if (!selected) return;
    if (!window.confirm(`确定删除配置项「${key}」？`)) return;
    setError(null);
    try {
      await cluster.deleteConfigMapEntry(project.id, workspaceId, selected.name, key);
      await load();
    } catch (err) {
      setError(formatConfigMapError(err, "删除配置数据失败"));
    }
  }

  const entries = Object.entries(selected?.data ?? {});
  const showEmpty = !loading && !error && items.length === 0;

  return (
    <div className="project-configmap-page">
      <header className="project-configmap-toolbar">
        <div>
          <h4 className="project-configmap-title">ConfigMap</h4>
          <p className="project-configmap-subtitle">管理当前应用命名空间内的配置项，支持表格与 YAML 两种编辑方式。</p>
        </div>
        <div className="project-configmap-toolbar-actions">
          <button type="button" className="btn btn-secondary text-sm" onClick={() => void load()} disabled={loading}>
            刷新
          </button>
          <button
            type="button"
            className="btn btn-primary text-sm"
            onClick={openCreateModal}
            disabled={Boolean(error) && items.length === 0}
            title={error && items.length === 0 ? error : undefined}
          >
            + 创建
          </button>
        </div>
      </header>

      {loading ? <p className="project-configmap-loading">ConfigMap 加载中…</p> : null}

      {error ? (
        <div className="project-configmap-alert">
          <p>{error}</p>
          <button type="button" className="btn btn-secondary text-sm" onClick={() => void load()}>
            重试
          </button>
        </div>
      ) : null}

      {showEmpty ? (
        <div className="project-configmap-empty">
          <strong>还没有 ConfigMap</strong>
          <p>点击右上角「创建」，用表格或 YAML 一次性写入 Key / Value。</p>
        </div>
      ) : null}

      {!error && items.length > 1 ? (
        <div className="project-configmap-names" role="tablist" aria-label="ConfigMap 列表">
          {items.map((item) => (
            <button
              key={item.name}
              type="button"
              role="tab"
              aria-selected={item.name === selected?.name}
              className={`project-configmap-name-chip${item.name === selected?.name ? " is-active" : ""}`}
              onClick={() => {
                setSelectedName(item.name);
                setDetailYamlDirty(false);
                setDetailError(null);
              }}
            >
              {item.name}
              <span>{Object.keys(item.data ?? {}).length} 项</span>
            </button>
          ))}
        </div>
      ) : null}

      {!error && selected ? (
        <section className="project-configmap-card">
          <div className="project-configmap-meta">
            <div className="project-configmap-meta-item">
              <span>名称</span>
              <strong>{selected.name}</strong>
            </div>
            <div className="project-configmap-meta-item">
              <span>命名空间</span>
              <strong>{selected.namespace}</strong>
            </div>
            <div className="project-configmap-meta-item">
              <span>创建时间</span>
              <strong>{selected.created_at || "-"}</strong>
            </div>
            <div className="project-configmap-meta-item">
              <span>配置项</span>
              <strong>{entries.length}</strong>
            </div>
          </div>

          <div className="project-configmap-data-head">
            <div>
              <h5>数据列表</h5>
              <p>可视化表格或 YAML 源码编辑，保存 YAML 会覆盖全部 data。</p>
            </div>
            <div className="project-configmap-data-actions">
              <div className="project-configmap-seg" role="tablist" aria-label="数据展示方式">
                <button
                  type="button"
                  className={detailMode === "visual" ? "is-active" : ""}
                  onClick={() => switchDetailMode("visual")}
                >
                  可视化
                </button>
                <button
                  type="button"
                  className={detailMode === "yaml" ? "is-active" : ""}
                  onClick={() => switchDetailMode("yaml")}
                >
                  YAML
                </button>
              </div>
              {detailMode === "yaml" ? (
                <button
                  type="button"
                  className="btn btn-primary text-sm"
                  disabled={saving || !detailYamlDirty}
                  onClick={() => void handleSaveDetailYaml()}
                >
                  {saving ? "保存中…" : "保存 YAML"}
                </button>
              ) : (
                <button
                  type="button"
                  className="btn btn-secondary text-sm"
                  onClick={() => {
                    setEditingKey(null);
                    setEntryKey("");
                    setEntryValue("");
                    setFormError(null);
                    setEntryOpen(true);
                  }}
                >
                  + 添加配置数据
                </button>
              )}
            </div>
          </div>

          {detailError ? <p className="project-configmap-inline-error">{detailError}</p> : null}

          {detailMode === "yaml" ? (
            <textarea
              className="input project-configmap-yaml"
              rows={18}
              spellCheck={false}
              aria-label="ConfigMap YAML"
              value={detailYaml}
              onChange={(event) => {
                setDetailYaml(event.target.value);
                setDetailYamlDirty(true);
              }}
            />
          ) : (
            <div className="project-configmap-table-wrap">
              <table className="project-configmap-table">
                <thead>
                  <tr>
                    <th style={{ width: "22%" }}>Key</th>
                    <th>Value</th>
                    <th style={{ width: 120 }}>操作</th>
                  </tr>
                </thead>
                <tbody>
                  {entries.length === 0 ? (
                    <tr>
                      <td colSpan={3} className="project-configmap-table-empty">
                        暂无配置数据，点击右上角添加。
                      </td>
                    </tr>
                  ) : (
                    entries.map(([key, value]) => (
                      <tr key={key}>
                        <td>
                          <code className="project-configmap-key">{key}</code>
                        </td>
                        <td>
                          <pre className="project-configmap-value">{value || "—"}</pre>
                        </td>
                        <td>
                          <div className="project-configmap-row-actions">
                            <button
                              type="button"
                              className="project-runtime-pod-link"
                              onClick={() => {
                                setEditingKey(key);
                                setEntryKey(key);
                                setEntryValue(value);
                                setFormError(null);
                                setEntryOpen(true);
                              }}
                            >
                              编辑
                            </button>
                            <button
                              type="button"
                              className="project-runtime-pod-link is-danger"
                              onClick={() => void handleDeleteEntry(key)}
                            >
                              删除
                            </button>
                          </div>
                        </td>
                      </tr>
                    ))
                  )}
                </tbody>
              </table>
            </div>
          )}
        </section>
      ) : null}

      <Modal
        open={createOpen}
        onClose={() => setCreateOpen(false)}
        title="创建 ConfigMap"
        size="lg"
        description={`将创建到命名空间 ${namespace}，并以 app=${project.name} 标记归属。`}
        footer={
          <div className="modal-footer-actions">
            <button type="button" className="btn btn-secondary" onClick={() => setCreateOpen(false)} disabled={saving}>
              取消
            </button>
            <button type="button" className="btn btn-primary" onClick={() => void handleCreate()} disabled={saving}>
              {saving ? "创建中…" : "创建"}
            </button>
          </div>
        }
      >
        <div className="stack-md">
          {formError ? <p className="text-sm text-[var(--danger)]">{formError}</p> : null}
          <label className="stack-sm">
            <span>名称</span>
            <input
              className="input"
              value={createName}
              onChange={(event) => setCreateName(event.target.value)}
              placeholder={project.name}
            />
          </label>
          <div className="project-configmap-seg" role="tablist" aria-label="编辑方式">
            <button
              type="button"
              className={createMode === "visual" ? "is-active" : ""}
              onClick={() => switchCreateMode("visual")}
            >
              可视化
            </button>
            <button
              type="button"
              className={createMode === "yaml" ? "is-active" : ""}
              onClick={() => switchCreateMode("yaml")}
            >
              YAML
            </button>
          </div>
          {createMode === "visual" ? (
            <ConfigMapKvTable
              rows={createRows}
              onChange={setCreateRows}
              onImportFiles={(files) => void importFilesToRows(files, "create")}
            />
          ) : (
            <label className="stack-sm">
              <span>YAML</span>
              <textarea
                className="input project-configmap-yaml"
                rows={16}
                spellCheck={false}
                value={createYaml}
                onChange={(event) => setCreateYaml(event.target.value)}
              />
            </label>
          )}
        </div>
      </Modal>

      <Modal
        open={entryOpen}
        onClose={() => setEntryOpen(false)}
        title={editingKey ? "编辑配置数据" : "添加配置数据"}
        footer={
          <div className="modal-footer-actions">
            <button type="button" className="btn btn-secondary" onClick={() => setEntryOpen(false)} disabled={saving}>
              取消
            </button>
            <button type="button" className="btn btn-primary" onClick={() => void handleSaveEntry()} disabled={saving}>
              {saving ? "保存中…" : "确定"}
            </button>
          </div>
        }
      >
        <div className="stack-sm">
          {formError ? <p className="text-sm text-[var(--danger)]">{formError}</p> : null}
          <label className="stack-sm">
            <span>Key</span>
            <input
              className="input"
              value={entryKey}
              disabled={Boolean(editingKey)}
              onChange={(event) => setEntryKey(event.target.value)}
            />
          </label>
          <div className="flex justify-end">
            <input
              ref={entryFileRef}
              type="file"
              hidden
              onChange={(event) => {
                if (event.target.files?.length) void importFilesToRows(event.target.files, "entry");
                event.target.value = "";
              }}
            />
            <button type="button" className="btn btn-secondary text-sm" onClick={() => entryFileRef.current?.click()}>
              从文件填入 Value
            </button>
          </div>
          <label className="stack-sm">
            <span>Value</span>
            <textarea
              className="input"
              rows={8}
              value={entryValue}
              onChange={(event) => setEntryValue(event.target.value)}
            />
          </label>
        </div>
      </Modal>
    </div>
  );
}
