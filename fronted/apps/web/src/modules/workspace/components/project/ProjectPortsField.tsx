"use client";

import { useEffect, useState } from "react";
import type { ProjectPort } from "@sparkops/contracts/workspace";

interface ProjectPortsFieldProps {
  ports: ProjectPort[];
  onChange: (ports: ProjectPort[]) => void;
}

const DEFAULT_PROTOCOL: NonNullable<ProjectPort["protocol"]> = "TCP";

function emptyPort(): ProjectPort {
  return { name: "", container_port: 0, protocol: DEFAULT_PROTOCOL };
}

function commitPort(port: ProjectPort): ProjectPort {
  const containerPort = Math.max(0, port.container_port || 0);
  return {
    ...port,
    container_port: containerPort,
    service_port: port.service_port ?? containerPort,
    protocol: port.protocol ?? DEFAULT_PROTOCOL,
  };
}

function committedPorts(ports: ProjectPort[]) {
  return ports.filter((port) => port.container_port > 0).map(commitPort);
}

function draftHasContent(port: ProjectPort) {
  return port.container_port > 0 || Boolean(port.name?.trim());
}

export function ProjectPortsField({ ports, onChange }: ProjectPortsFieldProps) {
  const saved = committedPorts(ports);
  const [draft, setDraft] = useState<ProjectPort>(emptyPort);

  useEffect(() => {
    if (saved.length === 0) {
      setDraft(emptyPort());
    }
  }, [ports, saved.length]);

  // 首行草稿有有效端口时同步到表单，避免点「下一步」时端口未提交
  useEffect(() => {
    if (saved.length > 0 || draft.container_port <= 0) return;
    onChange([commitPort(draft)]);
    setDraft(emptyPort());
  }, [draft.container_port, draft.name, draft.protocol, draft.service_port, saved.length, onChange]);

  function replaceSaved(next: ProjectPort[]) {
    onChange(next.filter((port) => port.container_port > 0).map(commitPort));
  }

  function updateSavedRow(index: number, patch: Partial<ProjectPort>) {
    replaceSaved(saved.map((port, i) => (i === index ? commitPort({ ...port, ...patch }) : port)));
  }

  function updateDraft(patch: Partial<ProjectPort>) {
    setDraft((prev) => ({ ...prev, ...patch }));
  }

  function appendDraft() {
    const merged = commitPort(draft);
    if (merged.container_port <= 0) return;
    replaceSaved([...saved, merged]);
    setDraft(emptyPort());
  }

  function addRow(afterIndex: number) {
    const nextPort = commitPort({ name: "", container_port: 8080, protocol: DEFAULT_PROTOCOL });
    if (afterIndex >= saved.length - 1) {
      replaceSaved([...saved, nextPort]);
      return;
    }
    const next = [...saved];
    next.splice(afterIndex + 1, 0, nextPort);
    replaceSaved(next);
  }

  function removeRow(index: number) {
    replaceSaved(saved.filter((_, i) => i !== index));
  }

  function clearDraft() {
    setDraft(emptyPort());
  }

  function stepPort(index: number, delta: number, isDraft: boolean) {
    const current = isDraft ? draft : saved[index] ?? emptyPort();
    const base = current.container_port > 0 ? current.container_port : 8080;
    const next = Math.min(65535, Math.max(80, base + delta));
    if (isDraft) {
      updateDraft({ container_port: next });
      return;
    }
    updateSavedRow(index, { container_port: next });
  }

  function renderRow(port: ProjectPort, index: number, isDraft: boolean) {
    return (
      <div key={isDraft ? "draft" : index} className="project-ports-row">
        <div className="project-ports-port-wrap">
          <button
            type="button"
            className="project-ports-stepper-btn"
            aria-label="减少端口"
            onClick={() => stepPort(index, -1, isDraft)}
          >
            −
          </button>
          <input
            className="input project-ports-port-input"
            type="number"
            min={80}
            max={65535}
            placeholder="端口: 80 ~ 65535..."
            value={port.container_port > 0 ? port.container_port : ""}
            onChange={(e) => {
              const raw = e.target.value;
              const container_port = raw === "" ? 0 : Number(raw) || 0;
              if (isDraft) {
                updateDraft({ container_port });
                return;
              }
              updateSavedRow(index, { container_port });
            }}
          />
          <button
            type="button"
            className="project-ports-stepper-btn"
            aria-label="增加端口"
            onClick={() => stepPort(index, 1, isDraft)}
          >
            +
          </button>
        </div>
        <select
          className="input project-ports-protocol"
          value={port.protocol ?? DEFAULT_PROTOCOL}
          onChange={(e) => {
            const protocol = e.target.value as ProjectPort["protocol"];
            if (isDraft) {
              updateDraft({ protocol });
              return;
            }
            updateSavedRow(index, { protocol });
          }}
        >
          <option value="TCP">TCP</option>
          <option value="UDP">UDP</option>
        </select>
        <input
          className="input project-ports-name"
          placeholder="端口名称: http-80..."
          value={port.name ?? ""}
          onChange={(e) => {
            if (isDraft) {
              updateDraft({ name: e.target.value });
              return;
            }
            updateSavedRow(index, { name: e.target.value });
          }}
        />
        <button
          type="button"
          className="project-ports-add-btn"
          aria-label="添加端口"
          onClick={() => (isDraft ? appendDraft() : addRow(index))}
        >
          +
        </button>
        {!isDraft || draftHasContent(port) ? (
          <button
            type="button"
            className="project-ports-remove-btn"
            aria-label={isDraft ? "清除端口" : "删除端口"}
            onClick={() => (isDraft ? clearDraft() : removeRow(index))}
          >
            ×
          </button>
        ) : null}
      </div>
    );
  }

  return (
    <div className="project-ports-field">
      {saved.map((port, index) => renderRow(port, index, false))}
      {renderRow(draft, saved.length, true)}
    </div>
  );
}
