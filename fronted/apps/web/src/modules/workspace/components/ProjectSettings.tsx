"use client";

import { useState } from "react";
import { useRouter } from "next/navigation";
import { workspace } from "@sparkops/web-sdk";
import { BffError } from "@sparkops/contracts";
import type { ProjectVM } from "@sparkops/contracts/workspace";
import { ArchiveProjectModal } from "./project/ArchiveProjectModal";

interface EditProjectFormProps {
  workspaceId: string;
  project: ProjectVM;
}

export function EditProjectForm({ workspaceId, project }: EditProjectFormProps) {
  const router = useRouter();
  const [open, setOpen] = useState(false);
  const [name, setName] = useState(project.name);
  const [description, setDescription] = useState(project.description ?? "");
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  async function onSubmit(e: React.FormEvent) {
    e.preventDefault();
    setLoading(true);
    setError(null);
    try {
      await workspace.updateProject(workspaceId, project.id, { name, description });
      setOpen(false);
      router.refresh();
    } catch (err) {
      setError(err instanceof BffError ? err.message : "保存失败");
    } finally {
      setLoading(false);
    }
  }

  if (!open) {
    return (
      <button className="btn-secondary" type="button" onClick={() => setOpen(true)}>
        编辑项目
      </button>
    );
  }

  return (
    <form className="form-panel space-y-4" onSubmit={onSubmit}>
      <div className="flex items-center justify-between gap-4">
        <h3 className="section-title">编辑项目</h3>
        <button className="btn-ghost !py-1 !px-2" type="button" onClick={() => setOpen(false)}>
          取消
        </button>
      </div>
      <div>
        <label className="label" htmlFor="edit-project-name">
          名称
        </label>
        <input
          id="edit-project-name"
          className="input"
          value={name}
          onChange={(e) => setName(e.target.value)}
          required
        />
      </div>
      <div>
        <label className="label" htmlFor="edit-project-desc">
          描述
        </label>
        <textarea
          id="edit-project-desc"
          className="input min-h-20 resize-y"
          value={description}
          onChange={(e) => setDescription(e.target.value)}
        />
      </div>
      {error ? <p className="error">{error}</p> : null}
      <button className="btn" type="submit" disabled={loading}>
        {loading ? "保存中..." : "保存"}
      </button>
    </form>
  );
}

interface DeleteProjectButtonProps {
  workspaceId: string;
  projectId: string;
  projectName: string;
}

export function DeleteProjectButton({
  workspaceId,
  projectId,
  projectName,
}: DeleteProjectButtonProps) {
  const router = useRouter();
  const [open, setOpen] = useState(false);
  const [project, setProject] = useState<ProjectVM | null>(null);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  async function openModal() {
    setLoading(true);
    setError(null);
    try {
      setProject(await workspace.getProject(workspaceId, projectId));
      setOpen(true);
    } catch (err) {
      setError(err instanceof BffError ? err.message : "加载项目失败");
    } finally {
      setLoading(false);
    }
  }

  return (
    <>
      <div className="flex flex-col items-end gap-2">
        <button
          className="btn-secondary !text-red-600 !border-red-200 hover:!bg-red-50"
          type="button"
          onClick={() => void openModal()}
          disabled={loading}
          aria-label={`归档项目 ${projectName}`}
        >
          {loading ? "加载中…" : "删除项目"}
        </button>
        {error ? <p className="error">{error}</p> : null}
      </div>
      <ArchiveProjectModal
        open={open}
        workspaceId={workspaceId}
        project={project}
        onClose={() => setOpen(false)}
        onArchived={() => {
          router.push("/dashboard");
          router.refresh();
        }}
      />
    </>
  );
}
