"use client";

import { useMemo, useState } from "react";
import type { CreateProjectCommand, ProjectPort } from "@sparkops/contracts/workspace";
import { PROJECT_LANGUAGE_OPTIONS } from "@sparkops/contracts/workspace";
import type { SecretVM } from "@sparkops/contracts/secret";
import { Modal } from "@/components/ui/Modal";
import { FormRow } from "./project-form-shared";
import {
  CPU_MARKS,
  DEFAULT_GIT_BRANCHES,
  DOCKERFILE_SUGGESTIONS,
  MEMORY_MARKS,
  formatCpuMillicores,
  formatMemoryMi,
  parseCpuToMillicores,
  parseMemoryToMi,
} from "./project-resource-utils";

interface ProjectCreateStep2Props {
  form: CreateProjectCommand;
  updateForm: (patch: Partial<CreateProjectCommand>) => void;
  gitServiceType: "GitLab" | "GitHub";
  setGitServiceType: (type: "GitLab" | "GitHub") => void;
  gitSecrets: SecretVM[];
  loadingSecrets: boolean;
  loadSecrets: () => void;
}

function ResourceRangeField({
  title,
  icon,
  requestLabel,
  requestValue,
  limitValue,
  marks,
  format,
  parse,
  onRequestChange,
  onLimitChange,
}: {
  title: string;
  icon: string;
  requestLabel: string;
  requestValue: string;
  limitValue: string;
  marks: readonly { label: string; millicores?: number; mebibytes?: number }[];
  format: (value: number) => string;
  parse: (raw: string) => number;
  onRequestChange: (value: string) => void;
  onLimitChange: (value: string) => void;
}) {
  const requestNum = parse(requestValue);
  const limitNum = parse(limitValue);
  const min = marks[0]?.millicores ?? marks[0]?.mebibytes ?? 1;
  const max = marks[marks.length - 1]?.millicores ?? marks[marks.length - 1]?.mebibytes ?? 1000;

  function handleRequestSlider(value: number) {
    const next = Math.min(value, limitNum);
    onRequestChange(format(next));
  }

  function handleLimitSlider(value: number) {
    const next = Math.max(value, requestNum);
    onLimitChange(format(next));
  }

  return (
    <div className="project-resource-block">
      <div className="project-resource-block-head">
        <span className="project-resource-icon" aria-hidden>
          {icon}
        </span>
        <span className="project-resource-title">{title}</span>
        <span className="project-resource-info" title="设置容器资源请求与上限">
          i
        </span>
      </div>
      <div className="project-resource-inputs">
        <label>
          {requestLabel}:
          <input
            className="input project-resource-input"
            value={requestValue}
            onChange={(e) => onRequestChange(e.target.value)}
          />
        </label>
        <span className="project-resource-sep">~</span>
        <label>
          限制:
          <input
            className="input project-resource-input"
            value={limitValue}
            onChange={(e) => onLimitChange(e.target.value)}
          />
        </label>
      </div>
      <div className="project-resource-sliders">
        <input
          type="range"
          className="project-range"
          min={min}
          max={max}
          value={requestNum}
          onChange={(e) => handleRequestSlider(Number(e.target.value))}
        />
        <input
          type="range"
          className="project-range is-limit"
          min={min}
          max={max}
          value={limitNum}
          onChange={(e) => handleLimitSlider(Number(e.target.value))}
        />
      </div>
      <div className="project-resource-marks">
        {marks.map((mark) => {
          const value = mark.millicores ?? mark.mebibytes ?? 0;
          return (
            <button
              key={mark.label}
              type="button"
              className="project-resource-mark"
              onClick={() => {
                onRequestChange(format(value));
                onLimitChange(format(Math.min(value * 2, max)));
              }}
            >
              {mark.label}
            </button>
          );
        })}
      </div>
    </div>
  );
}

function gitRefLabel(refType?: string) {
  if (refType === "tag") return "标签名称";
  if (refType === "commit") return "提交 SHA";
  return "分支名称";
}

function gitRefPlaceholder(refType?: string) {
  if (refType === "tag") return "请输入标签名称，如：v1.0.0";
  if (refType === "commit") return "请输入提交 SHA";
  return "请输入分支名称，如：main, develop...";
}

function gitRefHint(refType?: string, manual?: boolean) {
  if (refType === "commit") return "输入 Git 提交的完整 SHA 值";
  if (refType === "tag") {
    return manual ? "手动输入 Git 标签名称" : "从 Git 仓库获取的标签列表，支持搜索";
  }
  return manual ? "手动输入 Git 分支名称" : "从 Git 仓库获取的分支列表，支持搜索";
}

export function ProjectCreateStep2({
  form,
  updateForm,
  gitServiceType,
  setGitServiceType,
  gitSecrets,
  loadingSecrets,
  loadSecrets,
}: ProjectCreateStep2Props) {
  const [gitRefMode, setGitRefMode] = useState<"select" | "manual">("select");
  const [gitRefQuery, setGitRefQuery] = useState("");
  const [portModalOpen, setPortModalOpen] = useState(false);
  const [configModalOpen, setConfigModalOpen] = useState(false);
  const [browseOpen, setBrowseOpen] = useState(false);

  const resources = form.resources ?? {};
  const ports = resources.ports ?? [];
  const refType = form.git_ref_type ?? "branch";
  const refManualOnly = refType === "commit";
  const refSelectMode = !refManualOnly && gitRefMode === "select";

  const branchOptions = useMemo(() => {
    const branchList = form.git_url?.trim() ? [...DEFAULT_GIT_BRANCHES] : [];
    const current = form.git_ref_value?.trim();
    const merged =
      current && !branchList.includes(current) ? [current, ...branchList] : branchList;
    if (!gitRefQuery.trim()) return merged;
    const q = gitRefQuery.trim().toLowerCase();
    return merged.filter((item) => item.toLowerCase().includes(q));
  }, [form.git_ref_value, form.git_url, gitRefQuery]);

  function updateResources(patch: Partial<NonNullable<CreateProjectCommand["resources"]>>) {
    updateForm({ resources: { ...resources, ...patch } });
  }

  function handleRefTypeChange(next: "branch" | "tag" | "commit") {
    updateForm({ git_ref_type: next, git_ref_value: next === "branch" ? "main" : "" });
    setGitRefMode(next === "commit" ? "manual" : "select");
    setGitRefQuery("");
  }

  return (
    <div className="project-form-section is-horizontal project-step2">
      <FormRow label="开发语言:" required>
        <select
          className="input"
          value={form.language ?? "go"}
          onChange={(e) => updateForm({ language: e.target.value })}
        >
          {PROJECT_LANGUAGE_OPTIONS.map((o) => (
            <option key={o.value} value={o.value}>
              {o.label}
            </option>
          ))}
        </select>
      </FormRow>

      <FormRow label="Git仓库地址:" required>
        <div className="project-input-with-icon">
          <span className="project-input-icon" aria-hidden>
            🔗
          </span>
          <input
            className="input"
            placeholder="请输入Git仓库地址"
            value={form.git_url ?? ""}
            onChange={(e) => updateForm({ git_url: e.target.value })}
          />
        </div>
      </FormRow>

      <FormRow label="Git服务类型:">
        <div className="project-radio-row">
          {(["GitLab", "GitHub"] as const).map((type) => (
            <label key={type} className="project-radio">
              <input
                type="radio"
                name="gitServiceType"
                checked={gitServiceType === type}
                onChange={() => {
                  setGitServiceType(type);
                  updateForm({ secret_id: "" });
                }}
              />
              {type}
            </label>
          ))}
        </div>
      </FormRow>

      <FormRow
        label="Git认证秘钥:"
        hint="若下拉列表中没有所需秘钥，请先在「配置管理 → 密钥配置」中创建并刷新；公开仓库可不选择秘钥。"
      >
        <select
          className="input"
          value={form.secret_id ?? ""}
          onFocus={loadSecrets}
          onChange={(e) => updateForm({ secret_id: e.target.value })}
        >
          <option value="">{loadingSecrets ? "加载中…" : "请选择Git认证秘钥（可选）"}</option>
          {gitSecrets.map((s) => (
            <option key={s.id} value={s.id}>
              {s.display_name || s.name} ({s.service_type})
            </option>
          ))}
        </select>
      </FormRow>

      <FormRow label="Git引用类型:" required>
        <div className="project-radio-row">
          {([
            ["branch", "分支 (Branch)"],
            ["tag", "标签 (Tag)"],
            ["commit", "某个提交 (Commit)"],
          ] as const).map(([value, label]) => (
            <label key={value} className="project-radio">
              <input
                type="radio"
                name="gitRefType"
                checked={refType === value}
                onChange={() => handleRefTypeChange(value)}
              />
              {label}
            </label>
          ))}
        </div>
      </FormRow>

      <FormRow
        label={`${gitRefLabel(refType)}:`}
        required
        hint={gitRefHint(refType, refManualOnly || gitRefMode === "manual")}
        labelExtra={
          !refManualOnly ? (
            <button
              type="button"
              className="project-manual-link"
              onClick={() => setGitRefMode((mode) => (mode === "select" ? "manual" : "select"))}
            >
              {gitRefMode === "select" ? "手动输入" : "从列表选择"}
            </button>
          ) : null
        }
      >
        {refSelectMode ? (
          <div className="project-git-ref-select">
            <input
              className="input"
              list="project-git-ref-options"
              placeholder={gitRefPlaceholder(refType)}
              value={gitRefQuery || form.git_ref_value || ""}
              onChange={(e) => {
                setGitRefQuery(e.target.value);
                updateForm({ git_ref_value: e.target.value });
              }}
              onFocus={() => setGitRefQuery(form.git_ref_value ?? "")}
            />
            <datalist id="project-git-ref-options">
              {branchOptions.map((item) => (
                <option key={item} value={item} />
              ))}
            </datalist>
          </div>
        ) : (
          <input
            className="input"
            placeholder={gitRefPlaceholder(refType)}
            value={form.git_ref_value ?? ""}
            onChange={(e) => updateForm({ git_ref_value: e.target.value })}
          />
        )}
      </FormRow>

      <FormRow
        label="Dockerfile路径:"
        required
        hint="Dockerfile 在 Git 仓库中的相对路径，默认为 ./Dockerfile。点击「浏览文件」可从常用路径中选择。"
      >
        <div className="project-input-group">
          <input
            className="input"
            placeholder="./Dockerfile"
            value={form.dockerfile_path ?? "./Dockerfile"}
            onChange={(e) => updateForm({ dockerfile_path: e.target.value })}
          />
          <button type="button" className="btn-secondary project-browse-btn" onClick={() => setBrowseOpen(true)}>
            浏览文件
          </button>
        </div>
      </FormRow>

      <div className="project-resource-card">
        <div className="project-resource-card-title">资源配置</div>
        <ResourceRangeField
          title="CPU 资源"
          icon="▣"
          requestLabel="请求"
          requestValue={resources.cpu_request ?? "500m"}
          limitValue={resources.cpu_limit ?? "1000m"}
          marks={CPU_MARKS}
          format={formatCpuMillicores}
          parse={parseCpuToMillicores}
          onRequestChange={(cpu_request) => updateResources({ cpu_request })}
          onLimitChange={(cpu_limit) => updateResources({ cpu_limit })}
        />
        <ResourceRangeField
          title="内存资源"
          icon="▤"
          requestLabel="请求"
          requestValue={resources.memory_request ?? "512Mi"}
          limitValue={resources.memory_limit ?? "1Gi"}
          marks={MEMORY_MARKS}
          format={formatMemoryMi}
          parse={parseMemoryToMi}
          onRequestChange={(memory_request) => updateResources({ memory_request })}
          onLimitChange={(memory_limit) => updateResources({ memory_limit })}
        />
        <div className="project-resource-inline">
          <label className="project-replicas-field">
            副本数
            <div className="project-stepper">
              <button
                type="button"
                className="project-stepper-btn"
                onClick={() =>
                  updateResources({ replicas: Math.max(1, (resources.replicas ?? 1) - 1) })
                }
              >
                −
              </button>
              <input
                className="input project-stepper-input"
                type="number"
                min={1}
                max={100}
                value={resources.replicas ?? 1}
                onChange={(e) =>
                  updateResources({ replicas: Math.max(1, Number(e.target.value) || 1) })
                }
              />
              <button
                type="button"
                className="project-stepper-btn"
                onClick={() =>
                  updateResources({ replicas: Math.min(100, (resources.replicas ?? 1) + 1) })
                }
              >
                +
              </button>
            </div>
          </label>
          <button type="button" className="btn-secondary project-config-btn" onClick={() => setPortModalOpen(true)}>
            配置端口
          </button>
        </div>
        <div className="project-resource-textareas">
          <label className="project-textarea-field">
            <span>Commands</span>
            <textarea
              className="input min-h-20"
              placeholder="/bin/sh&#10;-c"
              value={resources.command ?? ""}
              onChange={(e) => updateResources({ command: e.target.value })}
            />
            <small>覆盖容器镜像的 ENTRYPOINT，每行一个命令参数</small>
          </label>
          <label className="project-textarea-field">
            <span>Args</span>
            <textarea
              className="input min-h-20"
              placeholder="echo&#10;hello world"
              value={resources.args ?? ""}
              onChange={(e) => updateResources({ args: e.target.value })}
            />
            <small>覆盖容器镜像的 CMD，每行一个参数</small>
          </label>
        </div>
      </div>

      <div className="project-extra-config">
        <span className="project-extra-config-label">项目配置</span>
        <button type="button" className="btn-secondary project-config-btn" onClick={() => setConfigModalOpen(true)}>
          编辑配置
        </button>
        {resources.project_config?.trim() ? (
          <span className="project-extra-config-status">已配置</span>
        ) : null}
      </div>

      <Modal open={portModalOpen} onClose={() => setPortModalOpen(false)} title="配置端口" size="md">
        <div className="project-port-modal">
          {ports.length === 0 ? (
            <p className="project-form-hint">暂无端口，点击下方按钮添加。</p>
          ) : (
            ports.map((port, index) => (
              <div key={index} className="project-port-row">
                <input
                  className="input"
                  placeholder="名称"
                  value={port.name ?? ""}
                  onChange={(e) => {
                    const next = [...ports];
                    next[index] = { ...port, name: e.target.value };
                    updateResources({ ports: next });
                  }}
                />
                <input
                  className="input"
                  type="number"
                  placeholder="容器端口"
                  value={port.container_port || ""}
                  onChange={(e) => {
                    const next = [...ports];
                    next[index] = { ...port, container_port: Number(e.target.value) || 0 };
                    updateResources({ ports: next });
                  }}
                />
                <input
                  className="input"
                  type="number"
                  placeholder="服务端口"
                  value={port.service_port ?? ""}
                  onChange={(e) => {
                    const next = [...ports];
                    next[index] = {
                      ...port,
                      service_port: e.target.value ? Number(e.target.value) : undefined,
                    };
                    updateResources({ ports: next });
                  }}
                />
                <select
                  className="input"
                  value={port.protocol ?? "TCP"}
                  onChange={(e) => {
                    const next = [...ports];
                    next[index] = { ...port, protocol: e.target.value as ProjectPort["protocol"] };
                    updateResources({ ports: next });
                  }}
                >
                  <option value="TCP">TCP</option>
                  <option value="UDP">UDP</option>
                </select>
                <button
                  type="button"
                  className="project-action-btn is-delete"
                  onClick={() => updateResources({ ports: ports.filter((_, i) => i !== index) })}
                >
                  删除
                </button>
              </div>
            ))
          )}
          <button
            type="button"
            className="btn-secondary"
            onClick={() =>
              updateResources({
                ports: [...ports, { name: "", container_port: 8080, service_port: 8080, protocol: "TCP" }],
              })
            }
          >
            添加端口
          </button>
        </div>
      </Modal>

      <Modal open={configModalOpen} onClose={() => setConfigModalOpen(false)} title="项目配置" size="lg">
        <div className="project-config-modal">
          <p className="project-form-hint">可填写额外的 Kubernetes 配置（YAML），将在部署时合并到项目模板。</p>
          <textarea
            className="input min-h-48 font-mono text-sm"
            placeholder="apiVersion: v1&#10;kind: ConfigMap&#10;..."
            value={resources.project_config ?? ""}
            onChange={(e) => updateResources({ project_config: e.target.value })}
          />
        </div>
      </Modal>

      <Modal open={browseOpen} onClose={() => setBrowseOpen(false)} title="选择 Dockerfile 路径" size="md">
        <div className="project-browse-modal">
          {DOCKERFILE_SUGGESTIONS.map((path) => (
            <button
              key={path}
              type="button"
              className="project-browse-item"
              onClick={() => {
                updateForm({ dockerfile_path: path });
                setBrowseOpen(false);
              }}
            >
              {path}
            </button>
          ))}
          {!form.git_url?.trim() ? (
            <p className="project-form-hint">填写 Git 仓库地址后，可在创建项目后从仓库浏览更多文件。</p>
          ) : null}
        </div>
      </Modal>
    </div>
  );
}
