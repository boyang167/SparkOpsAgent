"use client";

import { useState } from "react";
import type { ProductProjectVM, WorkflowAdvancedVM, WorkflowTriggerVM, WorkflowVariableVM } from "@sparkops/contracts/product";
import {
  GIT_TRIGGER_EVENTS,
  SYSTEM_WORKFLOW_VARIABLES,
  emptyTrigger,
  emptyVariable,
  variableKeyError,
} from "./workflow-settings";

function CopyButton({ value }: { value: string }) {
  const [copied, setCopied] = useState(false);
  return (
    <button
      type="button"
      className="btn btn-ghost text-sm"
      onClick={() => {
        void navigator.clipboard?.writeText(value).then(() => {
          setCopied(true);
          window.setTimeout(() => setCopied(false), 1200);
        });
      }}
    >
      {copied ? "已复制" : "复制"}
    </button>
  );
}

export function WorkflowVariablesPanel({
  variables,
  onChange,
}: {
  variables: WorkflowVariableVM[];
  onChange: (variables: WorkflowVariableVM[]) => void;
}) {
  function patch(index: number, next: Partial<WorkflowVariableVM>) {
    onChange(variables.map((item, i) => (i === index ? { ...item, ...next } : item)));
  }

  return (
    <div className="pwf-build-form">
      <p className="text-sm text-muted">
        任务字段可写 {"{{.变量名}}"}。构建时会额外传入 Jenkins 参数 SPARKOPS_VAR_变量名，以及 JSON 参数 SPARKOPS_VARS。
      </p>
      {variables.length === 0 ? <p className="text-sm text-muted">还没有自定义变量。</p> : null}
      {variables.map((item, index) => {
        const error = item.key.trim() ? variableKeyError(item.key) : null;
        return (
          <div key={item.id} className="pwf-side-card">
            <label className="field">
              <span className="text-xs text-muted">变量名</span>
              <input className="input" value={item.key} onChange={(e) => patch(index, { key: e.target.value })} placeholder="IMAGE_TAG" />
            </label>
            {error ? <p className="text-xs text-danger">{error}</p> : null}
            <label className="field">
              <span className="text-xs text-muted">默认值</span>
              <input
                className="input"
                type={item.secret ? "password" : "text"}
                value={item.value}
                placeholder={item.secret && !item.value ? "留空表示保持原值" : undefined}
                onChange={(e) => patch(index, { value: e.target.value })}
              />
            </label>
            <label className="field">
              <span className="text-xs text-muted">Vault 路径</span>
              <input
                className="input"
                value={item.vault_path}
                placeholder="secret/data/app"
                onChange={(e) => patch(index, { vault_path: e.target.value })}
              />
            </label>
            <label className="field">
              <span className="text-xs text-muted">说明</span>
              <input className="input" value={item.description} onChange={(e) => patch(index, { description: e.target.value })} />
            </label>
            <div className="pwf-side-card-foot">
              <label className="pwf-check">
                <input
                  type="checkbox"
                  checked={item.runtime_input}
                  onChange={(e) => patch(index, { runtime_input: e.target.checked })}
                />
                运行时填写
              </label>
              <label className="pwf-check">
                <input
                  type="checkbox"
                  checked={item.secret}
                  onChange={(e) => patch(index, { secret: e.target.checked })}
                />
                加密保存
              </label>
              <button type="button" className="btn btn-ghost text-sm" onClick={() => onChange(variables.filter((_, i) => i !== index))}>
                删除
              </button>
            </div>
          </div>
        );
      })}
      <button type="button" className="btn btn-secondary text-sm" onClick={() => onChange([...variables, emptyVariable()])}>
        添加变量
      </button>
      <div className="stack-sm">
        <p className="text-sm font-medium">系统变量</p>
        <div className="pwf-sys-vars">
          {SYSTEM_WORKFLOW_VARIABLES.map((item) => (
            <span key={item.key} className="badge-neutral" title={item.label}>
              {`{{.${item.key}}}`}
            </span>
          ))}
        </div>
        <p className="text-xs text-muted">系统变量只读。非 Git 触发时，GIT_BRANCH、GIT_TAG、GIT_COMMIT、GIT_EVENT 为空。</p>
      </div>
    </div>
  );
}

export function WorkflowTriggersPanel({
  productId,
  workflowId,
  triggers,
  projects,
  onChange,
}: {
  productId: string;
  workflowId: string;
  triggers: WorkflowTriggerVM[];
  projects: ProductProjectVM[];
  onChange: (triggers: WorkflowTriggerVM[]) => void;
}) {
  function patch(index: number, next: Partial<WorkflowTriggerVM>) {
    onChange(triggers.map((item, i) => (i === index ? { ...item, ...next } : item)));
  }

  return (
    <div className="pwf-build-form">
      <p className="text-sm text-muted">手动执行始终可用。保存 Git 触发器后，会用项目仓库密钥在 GitLab 自动注册 Webhook；也可手动复制下面的地址和 Token。</p>
      {triggers.length === 0 ? <p className="text-sm text-muted">还没有触发器。</p> : null}
      {triggers.map((item, index) => {
        const enabled = item.enabled !== false;
        const gitPath = "/api/v1/public/workflows/gitlab";
        const hookPath = `/api/v1/products/${productId}/workflows/${workflowId}/hooks/${item.token}`;
        return (
          <div key={item.id} className={`pwf-side-card${enabled ? "" : " is-off"}`}>
            <div className="pwf-side-card-foot">
              <strong className="text-sm">{item.type === "git" ? "Git 触发" : item.type === "cron" ? "定时触发" : "Webhook"}</strong>
              <button
                type="button"
                className={`menu-switch${enabled ? " menu-switch-on" : ""}`}
                aria-pressed={enabled}
                aria-label="启用触发器"
                onClick={() => patch(index, { enabled: !enabled })}
              >
                <span className="menu-switch-thumb" />
              </button>
            </div>
            {item.type === "git" ? (
              <>
                <label className="field">
                  <span className="text-xs text-muted">项目</span>
                  <select className="input" value={item.project_id} onChange={(e) => patch(index, { project_id: e.target.value })}>
                    <option value="">请选择项目</option>
                    {projects.map((project) => (
                      <option key={project.id} value={project.id}>
                        {project.display_name || project.name}
                      </option>
                    ))}
                  </select>
                </label>
                <div className="stack-xs">
                  <span className="text-xs text-muted">事件</span>
                  <div className="pwf-deploy-checks">
                    {GIT_TRIGGER_EVENTS.map((event) => (
                      <label key={event.id} className="pwf-check">
                        <input
                          type="checkbox"
                          checked={(item.events ?? []).includes(event.id)}
                          onChange={(e) => {
                            const current = item.events ?? [];
                            const events = e.target.checked
                              ? [...current, event.id]
                              : current.filter((value) => value !== event.id);
                            patch(index, { events });
                          }}
                        />
                        {event.label}
                      </label>
                    ))}
                  </div>
                </div>
                <label className="field">
                  <span className="text-xs text-muted">分支 / Tag 过滤</span>
                  <input
                    className="input"
                    value={item.branch_filter}
                    placeholder="main, release/*，留空表示全部"
                    onChange={(e) => patch(index, { branch_filter: e.target.value })}
                  />
                </label>
                <label className="field">
                  <span className="text-xs text-muted">Webhook 路径</span>
                  <div className="pwf-copy-row">
                    <input className="input" readOnly value={gitPath} />
                    <CopyButton value={gitPath} />
                  </div>
                </label>
                <label className="field">
                  <span className="text-xs text-muted">X-Gitlab-Token</span>
                  <div className="pwf-copy-row">
                    <input className="input" readOnly value={item.token} />
                    <CopyButton value={item.token} />
                  </div>
                </label>
                <p className="text-xs text-muted">把路径接到后端地址后，例如 http://后端主机:8080{gitPath}。仓库地址需与所选项目的 Git 地址一致。</p>
              </>
            ) : null}
            {item.type === "webhook" ? (
              <>
                <label className="field">
                  <span className="text-xs text-muted">调用路径</span>
                  <div className="pwf-copy-row">
                    <input className="input" readOnly value={hookPath} />
                    <CopyButton value={hookPath} />
                  </div>
                </label>
                <p className="text-xs text-muted">POST 到后端该路径。Body 可选 branch、variables。</p>
              </>
            ) : null}
            {item.type === "cron" ? (
              <label className="field">
                <span className="text-xs text-muted">Cron 表达式</span>
                <input
                  className="input"
                  value={item.schedule}
                  placeholder="0 2 * * *"
                  onChange={(e) => patch(index, { schedule: e.target.value })}
                />
                <span className="text-xs text-muted">分 时 日 月 周。例如每天 02:00 为 0 2 * * *。</span>
              </label>
            ) : null}
            <div className="pwf-side-card-foot">
              <span />
              <button type="button" className="btn btn-ghost text-sm" onClick={() => onChange(triggers.filter((_, i) => i !== index))}>
                删除
              </button>
            </div>
          </div>
        );
      })}
      <div className="pwf-side-actions">
        <button type="button" className="btn btn-secondary text-sm" onClick={() => onChange([...triggers, emptyTrigger("git")])}>
          添加 Git
        </button>
        <button type="button" className="btn btn-secondary text-sm" onClick={() => onChange([...triggers, emptyTrigger("webhook")])}>
          添加 Webhook
        </button>
        <button type="button" className="btn btn-secondary text-sm" onClick={() => onChange([...triggers, emptyTrigger("cron")])}>
          添加定时
        </button>
      </div>
    </div>
  );
}

export function WorkflowAdvancedPanel({
  advanced,
  onChange,
}: {
  advanced: WorkflowAdvancedVM;
  onChange: (advanced: WorkflowAdvancedVM) => void;
}) {
  return (
    <div className="pwf-build-form">
      <label className="field">
        <span className="text-xs text-muted">并发</span>
        <select
          className="input"
          value={String(advanced.concurrency)}
          onChange={(e) => onChange({ ...advanced, concurrency: Number(e.target.value) })}
        >
          <option value="1">不允许并发</option>
          <option value="0">允许并发</option>
        </select>
      </label>
      <p className="text-xs text-muted">已有运行中的实例时，手动、Git、Webhook 和定时触发都会被拒绝。</p>
      <label className="field">
        <span className="text-xs text-muted">工作流超时（分钟）</span>
        <input
          className="input"
          type="number"
          min={0}
          value={advanced.timeout_minutes}
          onChange={(e) => onChange({ ...advanced, timeout_minutes: Math.max(0, Number(e.target.value) || 0) })}
        />
      </label>
      <p className="text-xs text-muted">0 表示不限制。超时后本次运行记为失败，已发出的 Jenkins 构建不会被中止。</p>
      <div className="pwf-side-card-foot">
        <span className="text-xs text-muted">调试断点</span>
        <button
          type="button"
          className={`menu-switch${advanced.debug ? " menu-switch-on" : ""}`}
          aria-pressed={advanced.debug}
          onClick={() => onChange({ ...advanced, debug: !advanced.debug })}
        >
          <span className="menu-switch-thumb" />
        </button>
      </div>
      <p className="text-xs text-muted">开启后每个阶段开始前暂停，需要手动继续。</p>
    </div>
  );
}
