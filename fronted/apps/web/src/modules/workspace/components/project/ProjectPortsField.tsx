"use client";

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

export function ProjectPortsField({ ports, onChange }: ProjectPortsFieldProps) {
  const saved = committedPorts(ports);
  const rows = [...saved, emptyPort()];

  function replaceSaved(next: ProjectPort[]) {
    onChange(next.filter((port) => port.container_port > 0).map(commitPort));
  }

  function updateRow(index: number, patch: Partial<ProjectPort>) {
    if (index < saved.length) {
      replaceSaved(saved.map((port, i) => (i === index ? commitPort({ ...port, ...patch }) : port)));
      return;
    }
    const merged = commitPort({ ...emptyPort(), ...patch });
    if (merged.container_port > 0) {
      replaceSaved([...saved, merged]);
    }
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

  function stepPort(index: number, delta: number) {
    const current = rows[index] ?? emptyPort();
    const base = current.container_port > 0 ? current.container_port : 8080;
    const next = Math.min(65535, Math.max(80, base + delta));
    updateRow(index, { container_port: next });
  }

  return (
    <div className="project-ports-field">
      {rows.map((port, index) => {
        const isSaved = index < saved.length;
        return (
          <div key={index} className="project-ports-row">
            <div className="project-ports-port-wrap">
              <button
                type="button"
                className="project-ports-stepper-btn"
                aria-label="减少端口"
                onClick={() => stepPort(index, -1)}
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
                  updateRow(index, {
                    container_port: raw === "" ? 0 : Number(raw) || 0,
                  });
                }}
              />
              <button
                type="button"
                className="project-ports-stepper-btn"
                aria-label="增加端口"
                onClick={() => stepPort(index, 1)}
              >
                +
              </button>
            </div>
            <select
              className="input project-ports-protocol"
              value={port.protocol ?? DEFAULT_PROTOCOL}
              onChange={(e) =>
                updateRow(index, { protocol: e.target.value as ProjectPort["protocol"] })
              }
            >
              <option value="TCP">TCP</option>
              <option value="UDP">UDP</option>
            </select>
            <input
              className="input project-ports-name"
              placeholder="端口名称: http-80..."
              value={port.name ?? ""}
              onChange={(e) => updateRow(index, { name: e.target.value })}
            />
            <button
              type="button"
              className="project-ports-add-btn"
              aria-label="添加端口"
              onClick={() => addRow(index)}
            >
              +
            </button>
            {isSaved ? (
              <button
                type="button"
                className="project-ports-remove-btn"
                aria-label="删除端口"
                onClick={() => removeRow(index)}
              >
                ×
              </button>
            ) : null}
          </div>
        );
      })}
    </div>
  );
}
