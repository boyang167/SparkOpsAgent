"use client";

import { useEffect, useMemo, useState } from "react";
import { product, secret } from "@sparkops/web-sdk";
import { BffError } from "@sparkops/contracts";
import type { SecretVM } from "@sparkops/contracts/secret";
import type { SprintExternalRef } from "@sparkops/contracts/product";
import { InlineAlert } from "@/components/ui/InlineAlert";
import { Modal } from "@/components/ui/Modal";
import type { ReleasePlanSprintOption } from "./ReleasePlanForm";

type LinkMode = "local" | "jira";

interface JiraSprintEntry {
  key: string;
  projectKey: string;
  boardId: string;
  sprintId: string;
  sprintName: string;
}

function newJiraEntry(): JiraSprintEntry {
  return {
    key: crypto.randomUUID(),
    projectKey: "",
    boardId: "",
    sprintId: "",
    sprintName: "",
  };
}

function jiraSecretLabel(item: SecretVM): string {
  const label = item.display_name || item.name;
  return `${item.service_url} (${label})`;
}

function isJiraSecret(item: SecretVM): boolean {
  return item.service_type.toLowerCase() === "jira" && item.status !== "inactive";
}

interface ReleasePlanLinkSprintModalProps {
  open: boolean;
  workspaceId: string;
  productId?: string;
  sprintOptions: ReleasePlanSprintOption[];
  selectedSprintId?: string;
  onClose: () => void;
  onConfirm: (sprintId: string) => void;
  onSprintCreated?: (sprint: ReleasePlanSprintOption) => void;
}

export function ReleasePlanLinkSprintModal({
  open,
  workspaceId,
  productId,
  sprintOptions,
  selectedSprintId,
  onClose,
  onConfirm,
  onSprintCreated,
}: ReleasePlanLinkSprintModalProps) {
  const [mode, setMode] = useState<LinkMode>("local");
  const [localSearch, setLocalSearch] = useState("");
  const [pickedLocalId, setPickedLocalId] = useState("");
  const [jiraSecrets, setJiraSecrets] = useState<SecretVM[]>([]);
  const [jiraSecretId, setJiraSecretId] = useState("");
  const [jiraEntries, setJiraEntries] = useState<JiraSprintEntry[]>([newJiraEntry()]);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    if (!open) return;
    setError(null);
    setLocalSearch("");
    setPickedLocalId(selectedSprintId ?? "");
    setJiraEntries([newJiraEntry()]);
    setMode("local");
    void secret
      .listSecrets(workspaceId)
      .then((items) => {
        const jiraItems = items.filter(isJiraSecret);
        setJiraSecrets(jiraItems);
        setJiraSecretId(jiraItems[0]?.id ?? "");
      })
      .catch(() => {
        setJiraSecrets([]);
        setJiraSecretId("");
      });
  }, [open, workspaceId, selectedSprintId]);

  const filteredLocal = useMemo(() => {
    const q = localSearch.trim().toLowerCase();
    if (!q) return sprintOptions;
    return sprintOptions.filter((item) => item.name.toLowerCase().includes(q));
  }, [localSearch, sprintOptions]);

  function updateJiraEntry(key: string, patch: Partial<JiraSprintEntry>) {
    setJiraEntries((prev) => prev.map((item) => (item.key === key ? { ...item, ...patch } : item)));
  }

  function removeJiraEntry(key: string) {
    setJiraEntries((prev) => (prev.length <= 1 ? prev : prev.filter((item) => item.key !== key)));
  }

  async function confirmLocal() {
    if (!pickedLocalId) {
      setError("请选择一个本地迭代");
      return;
    }
    onConfirm(pickedLocalId);
    onClose();
  }

  async function confirmJira() {
    if (!productId) {
      setError("请先选择目标产品");
      return;
    }
    if (jiraSecrets.length === 0) {
      setError("工作区未配置 Jira 凭据，请先在「密钥」中添加 service_type=Jira 的凭据");
      return;
    }
    const entry = jiraEntries.find((item) => item.projectKey.trim() && item.sprintId.trim());
    if (!entry) {
      setError("请填写项目与冲刺 ID");
      return;
    }
    const projectKey = entry.projectKey.trim();
    const jiraSprintId = entry.sprintId.trim();
    const jiraBoardId = entry.boardId.trim();
    const sprintName = entry.sprintName.trim() || `${projectKey} Sprint ${jiraSprintId}`;

    const existing = sprintOptions.find(
      (item) =>
        item.external?.provider === "jira" &&
        item.external.sprint_id === jiraSprintId &&
        (!item.external.project_key || item.external.project_key === projectKey),
    );
    if (existing) {
      onConfirm(existing.id);
      onClose();
      return;
    }

    setBusy(true);
    setError(null);
    try {
      const external: SprintExternalRef = {
        provider: "jira",
        project_key: projectKey,
        board_id: jiraBoardId || undefined,
        sprint_id: jiraSprintId,
      };
      const created = await product.createSprint(workspaceId, productId, {
        name: sprintName,
        external,
      });
      await product.importJiraSprint(workspaceId, productId, created.id, {
        jira_sprint_id: jiraSprintId,
        jira_board_id: jiraBoardId || undefined,
        sync_mode: "link_only",
      });
      const option: ReleasePlanSprintOption = {
        id: created.id,
        name: created.name,
        external: created.external,
      };
      onSprintCreated?.(option);
      onConfirm(created.id);
      onClose();
    } catch (err) {
      setError(err instanceof BffError ? err.message : "关联 Jira 冲刺失败");
    } finally {
      setBusy(false);
    }
  }

  async function onSubmit() {
    if (mode === "local") {
      await confirmLocal();
      return;
    }
    await confirmJira();
  }

  const jiraConfigured = jiraSecrets.length > 0;

  return (
    <Modal
      open={open}
      onClose={onClose}
      size="lg"
      title={mode === "jira" ? "关联 Jira 冲刺" : "关联冲刺"}
      description={mode === "jira" ? "从 Jira 导入冲刺并关联到发布计划。" : "选择已有本地迭代进行关联。"}
      footer={
        <>
          <button type="button" className="btn-secondary text-sm" disabled={busy} onClick={onClose}>
            取消
          </button>
          <button type="button" className="btn btn-primary text-sm" disabled={busy} onClick={() => void onSubmit()}>
            {busy ? "处理中…" : "确认"}
          </button>
        </>
      }
    >
      <div className="release-plan-link-sprint stack-md">
        {error ? <InlineAlert tone="error">{error}</InlineAlert> : null}

        <div className="release-plan-link-sprint-tabs" role="tablist">
          <button
            type="button"
            role="tab"
            aria-selected={mode === "local"}
            className={`release-plan-link-sprint-tab${mode === "local" ? " is-active" : ""}`}
            onClick={() => setMode("local")}
          >
            本地迭代
          </button>
          <button
            type="button"
            role="tab"
            aria-selected={mode === "jira"}
            className={`release-plan-link-sprint-tab${mode === "jira" ? " is-active" : ""}`}
            onClick={() => setMode("jira")}
          >
            Jira 冲刺
          </button>
        </div>

        {mode === "local" ? (
          <div className="stack-sm">
            <label className="form-field">
              <span className="form-label">搜索迭代</span>
              <input
                className="input"
                value={localSearch}
                onChange={(e) => setLocalSearch(e.target.value)}
                placeholder="按名称筛选"
              />
            </label>
            {filteredLocal.length === 0 ? (
              <p className="text-sm text-muted-foreground">暂无可用迭代，可切换到 Jira 冲刺导入。</p>
            ) : (
              <ul className="release-plan-link-sprint-list">
                {filteredLocal.map((item) => (
                  <li key={item.id}>
                    <label className="release-plan-link-sprint-option">
                      <input
                        type="radio"
                        name="local-sprint"
                        checked={pickedLocalId === item.id}
                        onChange={() => setPickedLocalId(item.id)}
                      />
                      <span className="release-plan-link-sprint-option-body">
                        <span className="release-plan-link-sprint-option-title">{item.name}</span>
                        {item.external?.provider === "jira" ? (
                          <span className="release-plan-link-sprint-option-meta">
                            Jira · {item.external.project_key ?? "—"} · Sprint {item.external.sprint_id ?? "—"}
                          </span>
                        ) : null}
                      </span>
                    </label>
                  </li>
                ))}
              </ul>
            )}
          </div>
        ) : (
          <div className="stack-sm">
            {!productId ? (
              <InlineAlert tone="warning">请先在表单中选择目标产品，再关联 Jira 冲刺。</InlineAlert>
            ) : null}
            <label className="form-field">
              <span className="form-label">
                Jira 地址 <span className="text-red-600">*</span>
              </span>
              {jiraConfigured ? (
                <select className="input" value={jiraSecretId} onChange={(e) => setJiraSecretId(e.target.value)}>
                  {jiraSecrets.map((item) => (
                    <option key={item.id} value={item.id}>
                      {jiraSecretLabel(item)}
                    </option>
                  ))}
                </select>
              ) : (
                <p className="text-sm text-muted-foreground">
                  未配置 Jira 凭据。请在工作区「密钥」中添加 service_type 为 Jira 的凭据。
                </p>
              )}
              {jiraConfigured ? (
                <p className="field-hint text-xs text-muted-foreground">导入 Issue 时使用工作区已配置的 Jira 凭据。</p>
              ) : null}
            </label>

            {jiraEntries.map((entry, index) => (
              <div key={entry.key} className="release-plan-link-sprint-jira-block">
                <div className="release-plan-link-sprint-jira-block-head">
                  <span className="text-sm font-medium">冲刺 {index + 1}</span>
                  {jiraEntries.length > 1 ? (
                    <button
                      type="button"
                      className="btn-ghost text-xs text-red-600"
                      aria-label="删除"
                      onClick={() => removeJiraEntry(entry.key)}
                    >
                      删除
                    </button>
                  ) : null}
                </div>
                <div className="grid gap-3 sm:grid-cols-2">
                  <label className="form-field sm:col-span-2">
                    <span className="form-label">
                      项目 <span className="text-red-600">*</span>
                    </span>
                    <input
                      className="input"
                      value={entry.projectKey}
                      onChange={(e) => updateJiraEntry(entry.key, { projectKey: e.target.value })}
                      placeholder="例如 kr"
                    />
                  </label>
                  <label className="form-field">
                    <span className="form-label">面板</span>
                    <input
                      className="input"
                      value={entry.boardId}
                      onChange={(e) => updateJiraEntry(entry.key, { boardId: e.target.value })}
                      placeholder="请选择面板 ID"
                    />
                  </label>
                  <label className="form-field">
                    <span className="form-label">
                      冲刺 <span className="text-red-600">*</span>
                    </span>
                    <input
                      className="input"
                      value={entry.sprintId}
                      onChange={(e) => updateJiraEntry(entry.key, { sprintId: e.target.value })}
                      placeholder="请选择冲刺 ID"
                    />
                  </label>
                  <label className="form-field sm:col-span-2">
                    <span className="form-label">本地迭代名称</span>
                    <input
                      className="input"
                      value={entry.sprintName}
                      onChange={(e) => updateJiraEntry(entry.key, { sprintName: e.target.value })}
                      placeholder="留空将自动生成"
                    />
                  </label>
                </div>
              </div>
            ))}

            <button
              type="button"
              className="release-plan-link-sprint-add text-sm text-primary"
              onClick={() => setJiraEntries((prev) => [...prev, newJiraEntry()])}
            >
              + 添加
            </button>
            {jiraEntries.length > 1 ? (
              <p className="text-xs text-muted-foreground">发布计划当前仅支持关联一个迭代，将使用第一条有效配置。</p>
            ) : null}
          </div>
        )}
      </div>
    </Modal>
  );
}
