"use client";

import { useEffect, useState } from "react";
import type { DecideProjectReviewCommand, ProjectVM } from "@sparkops/contracts/workspace";
import { Modal } from "@/components/ui/Modal";
import { ProjectGeneratedTemplates } from "./ProjectGeneratedTemplates";
import { languageLabel } from "./project-utils";

const COMMENT_MAX = 500;

interface ProjectReviewModalProps {
  open: boolean;
  workspaceId: string;
  project: ProjectVM | null;
  loading?: boolean;
  onClose: () => void;
  onSubmit: (input: DecideProjectReviewCommand) => void;
}

function InfoField({ label, value }: { label: string; value: string }) {
  return (
    <div className="project-review-field">
      <span className="project-review-field-label">{label}</span>
      <span className="project-review-field-value">{value || "-"}</span>
    </div>
  );
}

export function ProjectReviewModal({
  open,
  workspaceId,
  project,
  loading = false,
  onClose,
  onSubmit,
}: ProjectReviewModalProps) {
  const [decision, setDecision] = useState<"approved" | "rejected">("approved");
  const [comment, setComment] = useState("");

  useEffect(() => {
    if (open) {
      setDecision("approved");
      setComment("");
    }
  }, [open, project?.id]);

  const rejectionMissingComment = decision === "rejected" && !comment.trim();
  const submitDisabled = loading || !project || rejectionMissingComment;

  return (
    <Modal
      open={open}
      onClose={onClose}
      title="项目审核"
      size="xl"
      panelClassName="project-review-modal"
      footer={
        <>
          <button type="button" className="btn-secondary" onClick={onClose} disabled={loading}>
            取消
          </button>
          <button
            type="button"
            className={`btn project-review-submit${decision === "rejected" ? " is-reject" : ""}`}
            disabled={submitDisabled}
            onClick={() => onSubmit({ decision, comment: comment.trim() })}
          >
            {loading ? "提交中…" : decision === "approved" ? "审核通过" : "审核拒绝"}
          </button>
        </>
      }
    >
      {project ? (
        <div className="project-review-body">
          <section className="project-review-section">
            <h3 className="project-review-section-title">项目信息</h3>
            <div className="project-review-info-grid">
              <InfoField label="项目名称" value={project.name} />
              <InfoField label="显示名称" value={project.display_name || project.name} />
              <InfoField label="命名空间" value={project.namespace || project.name} />
              <InfoField label="开发语言" value={languageLabel(project.language)} />
              <div className="project-review-field is-full">
                <span className="project-review-field-label">Git 仓库</span>
                <span className="project-review-field-value">{project.git_url || "-"}</span>
              </div>
              <InfoField label="构建路径" value={project.dockerfile_path?.trim() || "./Dockerfile"} />
            </div>
          </section>

          <section className="project-review-section">
            <h3 className="project-review-section-title">项目生成的模板</h3>
            <ProjectGeneratedTemplates workspaceId={workspaceId} project={project} variant="review" />
          </section>

          <section className="project-review-section">
            <h3 className="project-review-section-title">审核意见</h3>
            <div className="project-review-form">
              <div className="project-review-form-row">
                <span className="project-review-form-label">
                  <span className="project-review-required">*</span>
                  审核结果
                </span>
                <div className="project-review-radio-group" role="radiogroup" aria-label="审核结果">
                  <label className={`project-review-radio${decision === "approved" ? " is-checked" : ""}`}>
                    <input
                      type="radio"
                      name="review-decision"
                      value="approved"
                      checked={decision === "approved"}
                      disabled={loading}
                      onChange={() => setDecision("approved")}
                    />
                    <span>通过</span>
                  </label>
                  <label className={`project-review-radio${decision === "rejected" ? " is-checked" : ""}`}>
                    <input
                      type="radio"
                      name="review-decision"
                      value="rejected"
                      checked={decision === "rejected"}
                      disabled={loading}
                      onChange={() => setDecision("rejected")}
                    />
                    <span>拒绝</span>
                  </label>
                </div>
              </div>

              <div className="project-review-form-row is-vertical">
                <span className="project-review-form-label">
                  <span className="project-review-required">*</span>
                  审核意见
                </span>
                <div className="project-review-textarea-wrap">
                  <textarea
                    className="project-review-textarea"
                    rows={4}
                    maxLength={COMMENT_MAX}
                    value={comment}
                    disabled={loading}
                    placeholder="请输入审核意见..."
                    onChange={(event) => setComment(event.target.value)}
                  />
                  <span className="project-review-counter">
                    {comment.length} / {COMMENT_MAX}
                  </span>
                </div>
              </div>
            </div>
          </section>
        </div>
      ) : null}
    </Modal>
  );
}
