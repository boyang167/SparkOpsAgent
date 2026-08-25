"use client";

import { useEffect, useMemo, useState } from "react";
import type { GitTreeEntryVM } from "@sparkops/contracts/workspace";
import { BffError } from "@sparkops/contracts";
import { workspace } from "@sparkops/web-sdk";
import { Modal } from "@/components/ui/Modal";

interface DockerfilePickerModalProps {
  open: boolean;
  onClose: () => void;
  workspaceId: string;
  gitUrl: string;
  secretId: string;
  gitRef: string;
  gitServiceType: "GitLab" | "GitHub";
  onConfirm: (relativePath: string) => void;
}

function sortTreeEntries(items: GitTreeEntryVM[]) {
  return [...items].sort((a, b) => {
    const aIsDir = a.type === "tree";
    const bIsDir = b.type === "tree";
    if (aIsDir !== bIsDir) return aIsDir ? -1 : 1;
    return a.name.localeCompare(b.name);
  });
}

export function toRepoRelativePath(path: string) {
  const normalized = path.replace(/\\/g, "/").replace(/^\.\//, "");
  return `./${normalized}`;
}

function FolderIcon() {
  return (
    <svg viewBox="0 0 24 24" width="18" height="18" aria-hidden>
      <path
        fill="#e6a23c"
        d="M10 4l2 2h8a2 2 0 0 1 2 2v10a2 2 0 0 1-2 2H4a2 2 0 0 1-2-2V6a2 2 0 0 1 2-2h6z"
      />
    </svg>
  );
}

function FileIcon() {
  return (
    <svg viewBox="0 0 24 24" width="18" height="18" fill="none" stroke="#909399" strokeWidth="1.6" aria-hidden>
      <path d="M14 3H7a2 2 0 0 0-2 2v14a2 2 0 0 0 2 2h10a2 2 0 0 0 2-2V8l-5-5z" />
      <path d="M14 3v5h5" />
    </svg>
  );
}

export function DockerfilePickerModal({
  open,
  onClose,
  workspaceId,
  gitUrl,
  secretId,
  gitRef,
  gitServiceType,
  onConfirm,
}: DockerfilePickerModalProps) {
  const [browsePath, setBrowsePath] = useState("");
  const [selectedPath, setSelectedPath] = useState("");
  const [treeEntries, setTreeEntries] = useState<GitTreeEntryVM[]>([]);
  const [loadingTree, setLoadingTree] = useState(false);
  const [treeError, setTreeError] = useState<string | null>(null);

  const trimmedGitUrl = gitUrl.trim();
  const trimmedSecretId = secretId.trim();
  const trimmedGitRef = gitRef.trim();
  const canBrowse =
    Boolean(trimmedGitUrl && trimmedSecretId && trimmedGitRef) && gitServiceType === "GitLab";

  useEffect(() => {
    if (!open) return;
    setBrowsePath("");
    setSelectedPath("");
    setTreeError(null);
  }, [open]);

  useEffect(() => {
    if (!open || !workspaceId || !canBrowse) {
      setTreeEntries([]);
      return;
    }

    let cancelled = false;
    setLoadingTree(true);
    setTreeError(null);
    void workspace
      .listGitTree(workspaceId, {
        git_url: trimmedGitUrl,
        secret_id: trimmedSecretId,
        ref: trimmedGitRef,
        path: browsePath,
      })
      .then((items) => {
        if (cancelled) return;
        setTreeEntries(sortTreeEntries(items));
      })
      .catch((err: unknown) => {
        if (cancelled) return;
        setTreeEntries([]);
        setTreeError(
          err instanceof BffError
            ? err.message
            : err instanceof Error
              ? err.message
              : "加载仓库文件失败",
        );
      })
      .finally(() => {
        if (!cancelled) setLoadingTree(false);
      });

    return () => {
      cancelled = true;
    };
  }, [open, browsePath, workspaceId, canBrowse, trimmedGitUrl, trimmedSecretId, trimmedGitRef]);

  const breadcrumbs = useMemo(() => {
    if (!browsePath.trim()) return [];
    return browsePath.split("/").filter(Boolean);
  }, [browsePath]);

  function openFolder(path: string) {
    setBrowsePath(path);
    setSelectedPath("");
  }

  function handleConfirm() {
    if (!selectedPath.trim()) return;
    onConfirm(toRepoRelativePath(selectedPath));
    onClose();
  }

  const selectedDisplay = selectedPath ? toRepoRelativePath(selectedPath) : "未选择";

  return (
    <Modal
      open={open}
      onClose={onClose}
      title="选择 Dockerfile"
      size="md"
      panelClassName="project-dockerfile-picker-panel"
      footer={
        <div className="project-dockerfile-picker-footer">
          <div className="project-dockerfile-picker-status">
            已选择：
            <span className={selectedPath ? "is-set" : "is-empty"}>{selectedDisplay}</span>
          </div>
          <div className="modal-footer-actions">
            <button type="button" className="btn-secondary" onClick={onClose}>
              取消
            </button>
            <button
              type="button"
              className="btn project-btn-primary"
              disabled={!selectedPath.trim()}
              onClick={handleConfirm}
            >
              确定
            </button>
          </div>
        </div>
      }
    >
      <div className="project-dockerfile-picker">
        {!trimmedGitUrl ? (
          <p className="project-form-hint">请先填写 Git 仓库地址。</p>
        ) : !trimmedSecretId ? (
          <p className="project-form-hint">请先选择 Git 认证秘钥。</p>
        ) : !trimmedGitRef ? (
          <p className="project-form-hint">请先选择分支、标签或提交。</p>
        ) : gitServiceType !== "GitLab" ? (
          <p className="project-form-hint">当前仅 GitLab 支持浏览仓库文件。</p>
        ) : (
          <>
            <div className="project-dockerfile-picker-breadcrumb">
              <button type="button" className="project-link-btn" onClick={() => openFolder("")}>
                根目录
              </button>
              {breadcrumbs.map((part, index) => {
                const target = breadcrumbs.slice(0, index + 1).join("/");
                return (
                  <span key={target} className="project-dockerfile-picker-crumb">
                    <span className="project-dockerfile-picker-crumb-sep">/</span>
                    <button type="button" className="project-link-btn" onClick={() => openFolder(target)}>
                      {part}
                    </button>
                  </span>
                );
              })}
            </div>

            <div className="project-dockerfile-picker-list">
              {loadingTree ? <p className="project-form-hint">正在加载仓库文件…</p> : null}
              {!loadingTree && treeError ? <p className="project-form-hint">{treeError}</p> : null}
              {!loadingTree && !treeError && treeEntries.length === 0 ? (
                <p className="project-form-hint">当前目录为空。</p>
              ) : null}
              {!loadingTree
                ? treeEntries.map((item) => {
                    const isSelected = item.type !== "tree" && selectedPath === item.path;
                    return (
                      <button
                        key={item.path}
                        type="button"
                        className={`project-dockerfile-picker-row${isSelected ? " is-selected" : ""}`}
                        onClick={() =>
                          item.type === "tree" ? openFolder(item.path) : setSelectedPath(item.path)
                        }
                      >
                        <span className="project-dockerfile-picker-row-main">
                          <span className="project-dockerfile-picker-row-icon" aria-hidden>
                            {item.type === "tree" ? <FolderIcon /> : <FileIcon />}
                          </span>
                          <span>{item.name}</span>
                        </span>
                        {isSelected ? (
                          <span className="project-dockerfile-picker-tag">{item.name}</span>
                        ) : null}
                      </button>
                    );
                  })
                : null}
            </div>
          </>
        )}
      </div>
    </Modal>
  );
}
