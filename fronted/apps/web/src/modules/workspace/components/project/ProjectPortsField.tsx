"use client";

import { useState } from "react";
import type { ProjectPort } from "@sparkops/contracts/workspace";

interface ProjectPortsFieldProps {
  ports: ProjectPort[];
  onChange: (ports: ProjectPort[]) => void;
}

const DEFAULT_PROTOCOL: NonNullable<ProjectPort["protocol"]> = "TCP";
const DEFAULT_PORT_NUMBER = 8080;
const DEFAULT_PORT_NAME = "http-80";

function defaultPort(): ProjectPort {
  return {
    name: DEFAULT_PORT_NAME,
    container_port: DEFAULT_PORT_NUMBER,
    protocol: DEFAULT_PROTOCOL,
  };
}

function emptyPort(): ProjectPort {
  return { name: "", container_port: 0, protocol: DEFAULT_PROTOCOL };
}

function normalizePort(port: ProjectPort): ProjectPort {
  const containerPort = Math.max(0, port.container_port || 0);
  return {
    ...port,
    container_port: containerPort,
    service_port: port.service_port ?? containerPort,
    protocol: port.protocol ?? DEFAULT_PROTOCOL,
  };
}

function normalizePorts(ports: ProjectPort[]): ProjectPort[] {
  return ports.filter((port) => port.container_port > 0).map(normalizePort);
}

function portHasContent(port: ProjectPort) {
  return port.container_port > 0 || Boolean(port.name?.trim());
}

export function ProjectPortsField({ ports, onChange }: ProjectPortsFieldProps) {
  const rows = normalizePorts(ports);
  const [trailing, setTrailing] = useState<ProjectPort>(emptyPort);

  function replaceRows(next: ProjectPort[]) {
    onChange(normalizePorts(next));
  }

  function updateRow(index: number, patch: Partial<ProjectPort>) {
    replaceRows(rows.map((row, i) => (i === index ? normalizePort({ ...row, ...patch }) : row)));
  }

  function removeRow(index: number) {
    replaceRows(rows.filter((_, i) => i !== index));
  }

  function insertRow(afterIndex: number, port: ProjectPort) {
    const next = [...rows];
    next.splice(afterIndex + 1, 0, normalizePort(port));
    replaceRows(next);
  }

  function appendRow(port: ProjectPort) {
    replaceRows([...rows, normalizePort(port)]);
  }

  function commitTrailing() {
    if (trailing.container_port <= 0) return;
    appendRow(trailing);
    setTrailing(emptyPort());
  }

  function updateTrailing(patch: Partial<ProjectPort>) {
    setTrailing((prev) => normalizePort({ ...prev, ...patch }));
  }

  function clearTrailing() {
    setTrailing(emptyPort());
  }

  function stepPort(port: ProjectPort, delta: number, onUpdate: (patch: Partial<ProjectPort>) => void) {
    const base = port.container_port > 0 ? port.container_port : DEFAULT_PORT_NUMBER;
    onUpdate({ container_port: Math.min(65535, Math.max(80, base + delta)) });
  }

  function renderRow(
    port: ProjectPort,
    index: number,
    options: {
      key: string;
      onUpdate: (patch: Partial<ProjectPort>) => void;
      onAdd: () => void;
      onRemove: () => void;
      showRemove: boolean;
      onBlur?: () => void;
    },
  ) {
    const { key, onUpdate, onAdd, onRemove, showRemove, onBlur } = options;
    return (
      <div key={key} className="project-ports-row">
        <div className="project-ports-port-wrap">
          <button
            type="button"
            className="project-ports-stepper-btn"
            aria-label="减少端口"
            onClick={() => stepPort(port, -1, onUpdate)}
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
              onUpdate({ container_port: raw === "" ? 0 : Number(raw) || 0 });
            }}
            onBlur={onBlur}
          />
          <button
            type="button"
            className="project-ports-stepper-btn"
            aria-label="增加端口"
            onClick={() => stepPort(port, 1, onUpdate)}
          >
            +
          </button>
        </div>
        <select
          className="input project-ports-protocol"
          value={port.protocol ?? DEFAULT_PROTOCOL}
          onChange={(e) => onUpdate({ protocol: e.target.value as ProjectPort["protocol"] })}
        >
          <option value="TCP">TCP</option>
          <option value="UDP">UDP</option>
        </select>
        <input
          className="input project-ports-name"
          placeholder="端口名称: http-80..."
          value={port.name ?? ""}
          onChange={(e) => onUpdate({ name: e.target.value })}
        />
        <button type="button" className="project-ports-add-btn" aria-label="添加端口" onClick={onAdd}>
          +
        </button>
        {showRemove ? (
          <button type="button" className="project-ports-remove-btn" aria-label="删除端口" onClick={onRemove}>
            ×
          </button>
        ) : null}
      </div>
    );
  }

  return (
    <div className="project-ports-field">
      {rows.map((port, index) =>
        renderRow(port, index, {
          key: `saved-${index}`,
          onUpdate: (patch) => updateRow(index, patch),
          onAdd: () => insertRow(index, defaultPort()),
          onRemove: () => removeRow(index),
          showRemove: true,
        }),
      )}
      {renderRow(trailing, rows.length, {
        key: "trailing",
        onUpdate: (patch) => updateTrailing(patch),
        onAdd: () => {
          if (trailing.container_port > 0) {
            commitTrailing();
            return;
          }
          appendRow(defaultPort());
        },
        onRemove: () => clearTrailing(),
        showRemove: portHasContent(trailing),
        onBlur: () => {
          if (trailing.container_port > 0) {
            commitTrailing();
          }
        },
      })}
    </div>
  );
}
