"use client";

import { useEffect, useRef, useState } from "react";
import type { TemplateVM } from "@sparkops/contracts/product";
import {
  formatDateTime,
  labelOf,
  statusBadgeClass,
  TEMPLATE_KIND_OPTIONS,
  TEMPLATE_LANGUAGE_OPTIONS,
  TEMPLATE_STATUS_OPTIONS,
  truncateContent,
} from "./template-utils";
import { TemplateCreator } from "./TemplateCreator";

interface TemplateCardProps {
  template: TemplateVM;
  onView: (template: TemplateVM) => void;
  onEdit: (template: TemplateVM) => void;
  onCopy: (template: TemplateVM) => void;
  onDelete: (template: TemplateVM) => void;
}

export function TemplateCard({ template, onView, onEdit, onCopy, onDelete }: TemplateCardProps) {
  const [menuOpen, setMenuOpen] = useState(false);
  const menuRef = useRef<HTMLDivElement>(null);
  const editable = template.editable ?? !template.shared;

  useEffect(() => {
    if (!menuOpen) return;
    function onDocClick(e: MouseEvent) {
      if (!menuRef.current?.contains(e.target as Node)) setMenuOpen(false);
    }
    document.addEventListener("mousedown", onDocClick);
    return () => document.removeEventListener("mousedown", onDocClick);
  }, [menuOpen]);

  return (
    <article
      className="template-card"
      onClick={() => onView(template)}
      onKeyDown={(e) => {
        if (e.key === "Enter") onView(template);
      }}
      role="button"
      tabIndex={0}
    >
      <div className="template-card-header">
        <div className="template-card-info">
          <div className="template-card-name">
            <DocumentIcon />
            <span className="template-card-title">{template.name}</span>
          </div>
          <div className="template-card-meta">
            <span className={statusBadgeClass(template.status)}>
              {labelOf(TEMPLATE_STATUS_OPTIONS, template.status)}
            </span>
            <span className="badge badge-neutral">
              {labelOf(TEMPLATE_LANGUAGE_OPTIONS, template.language)}
            </span>
            {template.shared ? (
              <span className="badge badge-primary" title={template.workspace_name ?? "其他工作区"}>
                组织共享{template.workspace_name ? ` · ${template.workspace_name}` : ""}
              </span>
            ) : null}
          </div>
        </div>
        <div className="template-card-actions" ref={menuRef} onClick={(e) => e.stopPropagation()}>
          <button
            type="button"
            className="template-card-menu-btn"
            aria-label="更多操作"
            onClick={() => setMenuOpen((v) => !v)}
          >
            ⋮
          </button>
          {menuOpen ? (
            <div className="template-card-menu">
              <button type="button" onClick={() => { setMenuOpen(false); onView(template); }}>查看详情</button>
              {editable ? (
                <>
                  <button type="button" onClick={() => { setMenuOpen(false); onEdit(template); }}>编辑模板</button>
                  <button type="button" className="is-danger" onClick={() => { setMenuOpen(false); onDelete(template); }}>删除模板</button>
                </>
              ) : null}
              <button type="button" onClick={() => { setMenuOpen(false); onCopy(template); }}>复制到本工作区</button>
            </div>
          ) : null}
        </div>
      </div>

      <div className="template-card-body">
        <span className="badge badge-primary">{labelOf(TEMPLATE_KIND_OPTIONS, template.kind)}</span>
        <p className="template-card-desc">{truncateContent(template.detail)}</p>
      </div>

      <div className="template-card-footer">
        <TemplateCreator template={template} compact />
        <span className="template-card-time">{formatDateTime(template.created_at)}</span>
      </div>
    </article>
  );
}

function DocumentIcon() {
  return (
    <svg className="template-doc-icon" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
      <path d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8z" />
      <path d="M14 2v6h6M16 13H8M16 17H8M10 9H8" strokeLinecap="round" />
    </svg>
  );
}
