"use client";

import { useEffect, useRef, useState } from "react";
import {
  PRODUCT_WORKFLOW_JOB_CATEGORIES,
  PRODUCT_WORKFLOW_JOB_TYPES,
} from "@sparkops/contracts/product";
import { Modal } from "@/components/ui/Modal";

interface WorkflowJobPickerModalProps {
  open: boolean;
  onClose: () => void;
  onPick: (type: string) => void;
}

export function WorkflowJobPickerModal({ open, onClose, onPick }: WorkflowJobPickerModalProps) {
  const [activeCategory, setActiveCategory] = useState<string>(PRODUCT_WORKFLOW_JOB_CATEGORIES[0].id);

  useEffect(() => {
    if (open) setActiveCategory(PRODUCT_WORKFLOW_JOB_CATEGORIES[0].id);
  }, [open]);

  const mainRef = useRef<HTMLDivElement>(null);

  function focusCategory(categoryId: string) {
    setActiveCategory(categoryId);
    const root = mainRef.current;
    const section = document.getElementById(`pwf-job-cat-${categoryId}`);
    if (!root || !section) return;
    const top = section.getBoundingClientRect().top - root.getBoundingClientRect().top + root.scrollTop;
    root.scrollTo({ top, behavior: "smooth" });
  }

  return (
    <Modal
      open={open}
      onClose={onClose}
      title="选择任务"
      size="xl"
      panelClassName="pwf-job-catalog-modal"
    >
      <div className="pwf-job-catalog">
        <nav className="pwf-job-catalog-nav" aria-label="任务分类">
          {PRODUCT_WORKFLOW_JOB_CATEGORIES.map((category) => (
            <button
              key={category.id}
              type="button"
              className={activeCategory === category.id ? "is-active" : undefined}
              onClick={() => focusCategory(category.id)}
            >
              {category.label}
            </button>
          ))}
        </nav>
        <div className="pwf-job-catalog-main" ref={mainRef}>
          {PRODUCT_WORKFLOW_JOB_CATEGORIES.map((category) => {
            const jobs = PRODUCT_WORKFLOW_JOB_TYPES.filter((item) => item.category === category.id);
            return (
              <section key={category.id} id={`pwf-job-cat-${category.id}`} className="pwf-job-catalog-section">
                <h3 className="pwf-job-catalog-heading">{category.label}</h3>
                <div className="pwf-job-catalog-grid">
                  {jobs.map((job) => (
                    <button
                      key={job.type}
                      type="button"
                      className="pwf-job-card"
                      onClick={() => onPick(job.type)}
                    >
                      <span className="pwf-job-card-title">
                        {job.title}
                        {job.official ? <span className="pwf-job-official">官方</span> : null}
                      </span>
                      <span className="pwf-job-card-desc">{job.description}</span>
                    </button>
                  ))}
                </div>
              </section>
            );
          })}
        </div>
      </div>
    </Modal>
  );
}
