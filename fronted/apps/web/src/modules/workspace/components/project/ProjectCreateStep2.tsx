"use client";

import { useEffect, useMemo, useRef, useState } from "react";
import type { CreateProjectCommand, GitRefVM } from "@sparkops/contracts/workspace";
import { PROJECT_LANGUAGE_OPTIONS } from "@sparkops/contracts/workspace";
import type { SecretVM } from "@sparkops/contracts/secret";
import { BffError } from "@sparkops/contracts";
import { workspace } from "@sparkops/web-sdk";
import { Modal } from "@/components/ui/Modal";
import { FormRow } from "./project-form-shared";
import { DockerfilePickerModal } from "./DockerfilePickerModal";
import { ProjectPortsField } from "./ProjectPortsField";
import {
  parseProjectConfig,
  serializeProjectConfig,
  type ProjectConfigEntry,
} from "./project-utils";
import {
  CPU_MARKS,
  MEMORY_MARKS,
  formatCpuMillicores,
  formatMemoryMi,
  parseCpuToMillicores,
  parseMemoryToMi,
  snapToNearestMark,
} from "./project-resource-utils";
import { DualRangeSlider } from "./project-dual-slider";

const GIT_REF_UNSELECTED = "__unset__";

interface ProjectCreateStep2Props {
  workspaceId: string;
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
  hint,
  requestValue,
  limitValue,
  marks,
  format,
  parse,
  onRequestChange,
  onLimitChange,
}: {
  title: string;
  icon: React.ReactNode;
  hint: string;
  requestValue: string;
  limitValue: string;
  marks: readonly { label: string; millicores?: number; mebibytes?: number }[];
  format: (value: number) => string;
  parse: (raw: string) => number;
  onRequestChange: (value: string) => void;
  onLimitChange: (value: string) => void;
}) {
  const sliderMarks = marks.map((mark) => ({
    label: mark.label,
    value: mark.millicores ?? mark.mebibytes ?? 0,
  }));
  const markValues = sliderMarks.map((m) => m.value);
  const min = markValues[0] ?? 1;
  const max = markValues[markValues.length - 1] ?? 1000;
  const requestNum = Math.min(Math.max(parse(requestValue), min), max);
  const limitNum = Math.min(Math.max(parse(limitValue), min), max);

  return (
    <div className="project-resource-block">
      <div className="project-resource-block-head">
        <span className="project-resource-icon" aria-hidden>
          {icon}
        </span>
        <span className="project-resource-title">{title}</span>
        <span className="project-resource-info" title={hint}>
          i
        </span>
      </div>
      <div className="project-resource-inputs">
        <label>
          请求:
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
      <DualRangeSlider
        low={requestNum}
        high={Math.max(limitNum, requestNum)}
        marks={sliderMarks}
        onLowChange={(value) =>
          onRequestChange(format(snapToNearestMark(Math.round(value), markValues)))
        }
        onHighChange={(value) =>
          onLimitChange(format(snapToNearestMark(Math.round(value), markValues)))
        }
      />
    </div>
  );
}

function CpuIcon() {
  return (
    <svg viewBox="0 0 24 24" width="16" height="16" fill="none" stroke="currentColor" strokeWidth="1.8">
      <rect x="7" y="7" width="10" height="10" rx="1.5" />
      <path d="M9 3v4M12 3v4M15 3v4M9 17v4M12 17v4M15 17v4M3 9h4M3 12h4M3 15h4M17 9h4M17 12h4M17 15h4" />
    </svg>
  );
}

function MemoryIcon() {
  return (
    <svg viewBox="0 0 24 24" width="16" height="16" fill="none" stroke="currentColor" strokeWidth="1.8">
      <rect x="3" y="7" width="18" height="10" rx="1.5" />
      <path d="M7 7v10M11 7v10M15 7v10M6 17v2M10 17v2M14 17v2M18 17v2" />
    </svg>
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
  return "请输入分支名称，如：main, develop, feature/xxx";
}

function gitRefKindLabel(refType?: string) {
  if (refType === "tag") return "标签";
  if (refType === "commit") return "提交";
  return "分支";
}

function formatGitRefOption(item: GitRefVM) {
  if (item.title) {
    const short = item.name.length > 8 ? item.name.slice(0, 8) : item.name;
    const title = item.title.length > 60 ? `${item.title.slice(0, 60)}…` : item.title;
    return `${short} - ${title}`;
  }
  return item.name;
}

function gitRefHint(refType?: string, manual?: boolean) {
  if (refType === "commit") {
    return manual ? "手动输入 Git 提交的完整 SHA 值" : "从 Git 仓库获取的提交列表";
  }
  if (refType === "tag") {
    return manual ? "手动输入 Git 标签名称" : "从 Git 仓库获取的标签列表";
  }
  return manual ? "手动输入 Git 分支名称" : "从 Git 仓库获取的分支列表";
}

export function ProjectCreateStep2({
  workspaceId,
  form,
  updateForm,
  gitServiceType,
  setGitServiceType,
  gitSecrets,
  loadingSecrets,
  loadSecrets,
}: ProjectCreateStep2Props) {
  const [gitRefMode, setGitRefMode] = useState<"select" | "manual">("select");
  const [remoteRefs, setRemoteRefs] = useState<GitRefVM[]>([]);
  const [loadingGitRefs, setLoadingGitRefs] = useState(false);
  const [gitRefsError, setGitRefsError] = useState<string | null>(null);
  const [configModalOpen, setConfigModalOpen] = useState(false);
  const [configDraft, setConfigDraft] = useState<ProjectConfigEntry[]>([{ key: "", value: "" }]);
  const [browseOpen, setBrowseOpen] = useState(false);

  const resources = form.resources ?? {};
  const ports = resources.ports ?? [];
  const refType = form.git_ref_type ?? "branch";
  const refSelectMode = gitRefMode === "select";
  const gitRefContextRef = useRef<string | null>(null);

  useEffect(() => {
    const contextKey = `${form.git_url?.trim() ?? ""}|${form.secret_id?.trim() ?? ""}|${refType}`;
    if (gitRefContextRef.current === null) {
      gitRefContextRef.current = contextKey;
      return;
    }
    if (gitRefContextRef.current !== contextKey) {
      gitRefContextRef.current = contextKey;
      updateForm({ git_ref_value: "" });
    }
  }, [form.git_url, form.secret_id, refType, updateForm]);

  useEffect(() => {
    if (!refSelectMode || !workspaceId) {
      setRemoteRefs([]);
      setGitRefsError(null);
      return;
    }
    const gitUrl = form.git_url?.trim();
    const secretId = form.secret_id?.trim();
    if (!gitUrl || !secretId || gitServiceType !== "GitLab") {
      setRemoteRefs([]);
      return;
    }

    let cancelled = false;
    const timer = window.setTimeout(() => {
      setLoadingGitRefs(true);
      setGitRefsError(null);
      void workspace
        .listGitRefs(workspaceId, {
          git_url: gitUrl,
          secret_id: secretId,
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
          setGitRefsError(message);
        })
        .finally(() => {
          if (!cancelled) setLoadingGitRefs(false);
        });
    }, 400);

    return () => {
      cancelled = true;
      window.clearTimeout(timer);
    };
  }, [
    workspaceId,
    refSelectMode,
    form.git_url,
    form.secret_id,
    refType,
    gitServiceType,
  ]);

  const refOptions = useMemo(() => {
    const list = remoteRefs;
    const current = form.git_ref_value?.trim();
    if (current && !list.some((item) => item.name === current)) {
      return [{ name: current }, ...list];
    }
    return list;
  }, [form.git_ref_value, remoteRefs]);

  const selectedGitRef = form.git_ref_value?.trim() ? form.git_ref_value : GIT_REF_UNSELECTED;

  function updateResources(patch: Partial<NonNullable<CreateProjectCommand["resources"]>>) {
    updateForm({ resources: { ...resources, ...patch } });
  }

  function openConfigModal() {
    setConfigDraft(parseProjectConfig(resources.project_config));
    setConfigModalOpen(true);
  }

  function updateConfigDraft(index: number, patch: Partial<ProjectConfigEntry>) {
    setConfigDraft((prev) => prev.map((entry, i) => (i === index ? { ...entry, ...patch } : entry)));
  }

  function saveConfigDraft() {
    updateResources({ project_config: serializeProjectConfig(configDraft) });
    setConfigModalOpen(false);
  }

  function handleRefTypeChange(next: "branch" | "tag" | "commit") {
    updateForm({ git_ref_type: next, git_ref_value: "" });
    setGitRefMode("select");
  }

  return (
    <div className="project-form-section is-horizontal is-ref project-step2">
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
        hint="请先选择服务类型再刷新秘钥列表；公开仓库可不选择秘钥。"
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
        hint={gitRefHint(refType, gitRefMode === "manual")}
        labelExtra={
          <button
            type="button"
            className="project-manual-link"
            onClick={() => setGitRefMode((mode) => (mode === "select" ? "manual" : "select"))}
          >
            {gitRefMode === "select" ? "手动输入" : "下拉选择"}
          </button>
        }
      >
        {refSelectMode ? (
          <div className="project-git-ref-select">
            <select
              className="input"
              value={selectedGitRef}
              disabled={loadingGitRefs}
              onChange={(e) => {
                const next = e.target.value;
                updateForm({ git_ref_value: next === GIT_REF_UNSELECTED ? "" : next });
              }}
            >
              <option value={GIT_REF_UNSELECTED} disabled>
                {loadingGitRefs
                  ? `正在加载${gitRefKindLabel(refType)}…`
                  : refOptions.length
                    ? `请选择${gitRefKindLabel(refType)}`
                    : `暂无可用${gitRefKindLabel(refType)}`}
              </option>
              {refOptions.map((item) => (
                <option key={item.name} value={item.name}>
                  {formatGitRefOption(item)}
                </option>
              ))}
            </select>
            {loadingGitRefs ? (
              <p className="project-form-hint">正在从 Git 仓库加载{gitRefKindLabel(refType)}列表…</p>
            ) : null}
            {!loadingGitRefs && gitRefsError ? (
              <p className="project-form-hint">{gitRefsError}</p>
            ) : null}
            {!loadingGitRefs && !gitRefsError && !form.secret_id?.trim() ? (
              <p className="project-form-hint">请选择 Git 认证秘钥以从仓库加载列表</p>
            ) : null}
            {!loadingGitRefs && !gitRefsError && form.secret_id?.trim() && gitServiceType !== "GitLab" ? (
              <p className="project-form-hint">当前仅 GitLab 支持从仓库加载列表</p>
            ) : null}
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
        hint="Dockerfile在Git仓库中的相对路径，默认为 ./Dockerfile。点击「浏览文件」可从仓库中选择文件。"
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

      <FormRow
        label="端口及协议:"
        hint="如果不提供容器端口号，将不创建 service，外部无法直接访问，脚本或内部任务的容器可不填写"
      >
        <ProjectPortsField
          ports={ports}
          onChange={(next) => updateResources({ ports: next })}
        />
      </FormRow>

      <FormRow label="资源配置:">
        <div className="project-resource-card">
          <ResourceRangeField
            title="CPU 配置"
            icon={<CpuIcon />}
            hint="CPU 请求用于调度预留，限制为容器可用上限。单位 m 表示毫核，1000m = 1 核。"
            requestValue={resources.cpu_request ?? "500m"}
            limitValue={resources.cpu_limit ?? "1000m"}
            marks={CPU_MARKS}
            format={formatCpuMillicores}
            parse={parseCpuToMillicores}
            onRequestChange={(cpu_request) => updateResources({ cpu_request })}
            onLimitChange={(cpu_limit) => updateResources({ cpu_limit })}
          />
          <ResourceRangeField
            title="内存配置"
            icon={<MemoryIcon />}
            hint="内存请求用于调度预留，限制为容器可用上限。支持 Mi / Gi。"
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
          </div>
          <div className="project-resource-textareas">
            <label className="project-textarea-field">
              <span>Commands</span>
              <textarea
                className="input min-h-20"
                placeholder={"容器启动命令，每一行一个，如：\n/bin/sh\n-c"}
                value={resources.command ?? ""}
                onChange={(e) => updateResources({ command: e.target.value })}
              />
              <small>覆盖容器镜像的 ENTRYPOINT，每行一个命令参数</small>
            </label>
            <label className="project-textarea-field">
              <span>Args</span>
              <textarea
                className="input min-h-20"
                placeholder={"容器启动参数，每一行一个，如：\necho\nhello world"}
                value={resources.args ?? ""}
                onChange={(e) => updateResources({ args: e.target.value })}
              />
              <small>覆盖容器镜像的 CMD，每行一个参数</small>
            </label>
          </div>
        </div>
      </FormRow>

      <FormRow label="项目配置:" align="center">
        <div className="project-extra-config is-inline">
          <button type="button" className="project-link-btn" onClick={openConfigModal}>
            编辑配置
          </button>
          {resources.project_config?.trim() ? (
            <span className="project-extra-config-status">已配置</span>
          ) : null}
        </div>
      </FormRow>

      <Modal
        open={configModalOpen}
        onClose={() => setConfigModalOpen(false)}
        title="编辑项目配置"
        size="lg"
        panelClassName="project-kv-modal-panel"
        footer={
          <div className="modal-footer-actions">
            <button type="button" className="btn-secondary" onClick={() => setConfigModalOpen(false)}>
              取消
            </button>
            <button type="button" className="btn project-btn-primary" onClick={saveConfigDraft}>
              保存
            </button>
          </div>
        }
      >
        <div className="project-kv-modal">
          {configDraft.map((entry, index) => (
            <div key={index} className="project-kv-row">
              <input
                className="input"
                placeholder="配置键"
                value={entry.key}
                onChange={(e) => updateConfigDraft(index, { key: e.target.value })}
              />
              <input
                className="input"
                placeholder="配置值"
                value={entry.value}
                onChange={(e) => updateConfigDraft(index, { value: e.target.value })}
              />
              <button
                type="button"
                className="project-action-btn is-delete"
                onClick={() => setConfigDraft((prev) => prev.filter((_, i) => i !== index))}
              >
                删除
              </button>
            </div>
          ))}
          <button
            type="button"
            className="btn project-btn-primary project-kv-add"
            onClick={() => setConfigDraft((prev) => [...prev, { key: "", value: "" }])}
          >
            添加配置
          </button>
        </div>
      </Modal>

      <DockerfilePickerModal
        open={browseOpen}
        onClose={() => setBrowseOpen(false)}
        workspaceId={workspaceId}
        gitUrl={form.git_url ?? ""}
        secretId={form.secret_id ?? ""}
        gitRef={form.git_ref_value ?? ""}
        gitServiceType={gitServiceType}
        onConfirm={(path) => updateForm({ dockerfile_path: path })}
      />
    </div>
  );
}
