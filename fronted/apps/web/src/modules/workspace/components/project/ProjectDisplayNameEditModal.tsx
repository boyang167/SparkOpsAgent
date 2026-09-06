"use client";

import { useEffect, useState } from "react";
import { useRouter } from "next/navigation";
import { workspace } from "@sparkops/web-sdk";
import { BffError } from "@sparkops/contracts";
import type { ProjectVM } from "@sparkops/contracts/workspace";
import { Modal } from "@/components/ui/Modal";
import { InlineAlert } from "@/components/ui/InlineAlert";
import { projectDisplayName } from "./project-utils";

interface ProjectDisplayNameEditModalProps {
  open: boolean;
  workspaceId: string;
  project: ProjectVM;
  onClose: () => void;
  onSaved: (project: ProjectVM) => void;
}

export function ProjectDisplayNameEditModal({
  open,
  workspaceId,
  project,
  onClose,
  onSaved,
}: ProjectDisplayNameEditModalProps) {
  const router = useRouter();
  const [displayName, setDisplayName] = useState(project.display_name ?? project.name);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    if (!open) return;
    setDisplayName(project.display_name ?? project.name);
    setError(null);
  }, [open, project.display_name, project.name]);

  async function onSubmit(event: React.FormEvent) {
    event.preventDefault();
    setLoading(true);
    setError(null);
    try {
      const updated = await workspace.updateProject(workspaceId, project.id, {
        name: project.name,
        display_name: displayName.trim(),
        description: project.description ?? "",
      });
      onSaved(updated);
      onClose();
      router.refresh();
    } catch (err) {
      setError(err instanceof BffError ? err.message : "保存失败");
    } finally {
      setLoading(false);
    }
  }

  return (
    <Modal open={open} onClose={() => !loading && onClose()} title="修改项目中文名">
      <form className="stack-md" onSubmit={(event) => void onSubmit(event)}>
        <div>
          <label className="label" htmlFor="project-display-name">
            项目中文名
          </label>
          <input
            id="project-display-name"
            className="input"
            value={displayName}
            onChange={(event) => setDisplayName(event.target.value)}
            maxLength={50}
            required
            autoFocus
          />
        </div>
        <p className="text-sm text-muted">
          项目标识：<span className="font-mono">{project.name}</span>
        </p>
        {error ? <InlineAlert tone="error">{error}</InlineAlert> : null}
        <div className="flex justify-end gap-2">
          <button type="button" className="btn-secondary" disabled={loading} onClick={onClose}>
            取消
          </button>
          <button type="submit" className="btn project-btn-primary" disabled={loading}>
            {loading ? "保存中…" : "保存"}
          </button>
        </div>
      </form>
    </Modal>
  );
}

export function ProjectDescriptionEditModal({
  open,
  workspaceId,
  project,
  onClose,
  onSaved,
}: ProjectDisplayNameEditModalProps) {
  const router = useRouter();
  const [description, setDescription] = useState(project.description ?? "");
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    if (!open) return;
    setDescription(project.description ?? "");
    setError(null);
  }, [open, project.description]);

  async function onSubmit(event: React.FormEvent) {
    event.preventDefault();
    setLoading(true);
    setError(null);
    try {
      const updated = await workspace.updateProject(workspaceId, project.id, {
        name: project.name,
        display_name: project.display_name ?? project.name,
        description: description.trim(),
      });
      onSaved(updated);
      onClose();
      router.refresh();
    } catch (err) {
      setError(err instanceof BffError ? err.message : "保存失败");
    } finally {
      setLoading(false);
    }
  }

  return (
    <Modal open={open} onClose={() => !loading && onClose()} title="修改项目描述">
      <form className="stack-md" onSubmit={(event) => void onSubmit(event)}>
        <div>
          <label className="label" htmlFor="project-description">
            项目描述
          </label>
          <textarea
            id="project-description"
            className="input min-h-24 resize-y"
            value={description}
            onChange={(event) => setDescription(event.target.value)}
            maxLength={200}
            autoFocus
          />
        </div>
        {error ? <InlineAlert tone="error">{error}</InlineAlert> : null}
        <div className="flex justify-end gap-2">
          <button type="button" className="btn-secondary" disabled={loading} onClick={onClose}>
            取消
          </button>
          <button type="submit" className="btn project-btn-primary" disabled={loading}>
            {loading ? "保存中…" : "保存"}
          </button>
        </div>
      </form>
    </Modal>
  );
}

function EditIcon() {
  return (
    <svg viewBox="0 0 24 24" width="14" height="14" fill="none" stroke="currentColor" strokeWidth="2">
      <path d="M12 20h9" strokeLinecap="round" />
      <path d="M16.5 3.5a2.1 2.1 0 0 1 3 3L7 19l-4 1 1-4 12.5-12.5z" strokeLinecap="round" strokeLinejoin="round" />
    </svg>
  );
}

interface ProjectDisplayNameFieldProps {
  workspaceId: string;
  project: ProjectVM;
  onSaved: (project: ProjectVM) => void;
}

export function ProjectDisplayNameField({ workspaceId, project, onSaved }: ProjectDisplayNameFieldProps) {
  const [open, setOpen] = useState(false);

  return (
    <>
      <span className="project-meta-editable">
        <span>{projectDisplayName(project)}</span>
        <button
          type="button"
          className="project-meta-edit-btn"
          aria-label="修改项目中文名"
          onClick={() => setOpen(true)}
        >
          <EditIcon />
        </button>
      </span>
      <ProjectDisplayNameEditModal
        open={open}
        workspaceId={workspaceId}
        project={project}
        onClose={() => setOpen(false)}
        onSaved={onSaved}
      />
    </>
  );
}

export function ProjectDescriptionField({ workspaceId, project, onSaved }: ProjectDisplayNameFieldProps) {
  const [open, setOpen] = useState(false);
  const label = project.description?.trim() || "暂无描述";

  return (
    <>
      <span className="project-meta-editable">
        <span className={project.description?.trim() ? undefined : "text-muted"}>{label}</span>
        <button
          type="button"
          className="project-meta-edit-btn"
          aria-label="修改项目描述"
          onClick={() => setOpen(true)}
        >
          <EditIcon />
        </button>
      </span>
      <ProjectDescriptionEditModal
        open={open}
        workspaceId={workspaceId}
        project={project}
        onClose={() => setOpen(false)}
        onSaved={onSaved}
      />
    </>
  );
}
