"use client";

import { useState } from "react";
import { useRouter } from "next/navigation";
import { workspace } from "@sparkops/web-sdk";
import { BffError } from "@sparkops/contracts";

interface CreateProjectFormProps {
  workspaceId: string;
}

export function CreateProjectForm({ workspaceId }: CreateProjectFormProps) {
  const router = useRouter();
  const [open, setOpen] = useState(false);
  const [name, setName] = useState("");
  const [description, setDescription] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(false);

  async function onSubmit(e: React.FormEvent) {
    e.preventDefault();
    setLoading(true);
    setError(null);
    try {
      const project = await workspace.createProject(workspaceId, { name, description });
      setOpen(false);
      setName("");
      setDescription("");
      router.push(`/projects/${project.id}`);
      router.refresh();
    } catch (err) {
      setError(err instanceof BffError ? err.message : "创建失败");
    } finally {
      setLoading(false);
    }
  }

  if (!open) {
    return (
      <button className="btn" type="button" onClick={() => setOpen(true)}>
        新建项目
      </button>
    );
  }

  return (
    <form className="form-panel space-y-4" onSubmit={onSubmit}>
      <div className="flex items-center justify-between gap-4">
        <h3 className="section-title">新建项目</h3>
        <button className="btn-ghost !py-1 !px-2" type="button" onClick={() => setOpen(false)}>
          取消
        </button>
      </div>
      <div>
        <label className="label" htmlFor="project-name">
          名称
        </label>
        <input
          id="project-name"
          className="input"
          value={name}
          onChange={(e) => setName(e.target.value)}
          required
        />
      </div>
      <div>
        <label className="label" htmlFor="project-desc">
          描述
        </label>
        <textarea
          id="project-desc"
          className="input min-h-20 resize-y"
          value={description}
          onChange={(e) => setDescription(e.target.value)}
        />
      </div>
      {error ? <p className="error">{error}</p> : null}
      <button className="btn" type="submit" disabled={loading}>
        {loading ? "创建中..." : "创建"}
      </button>
    </form>
  );
}
