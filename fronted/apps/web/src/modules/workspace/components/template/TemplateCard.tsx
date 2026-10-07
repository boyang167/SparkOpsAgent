"use client";

import { useEffect, useRef, useState } from "react";
import type { TemplateVM } from "@sparkops/contracts/product";
import {
  formatDate,
  labelOf,
  previewLines,
  statusBadgeClass,
  TEMPLATE_KIND_OPTIONS,
  TEMPLATE_LANGUAGE_OPTIONS,
  TEMPLATE_STATUS_OPTIONS,
} from "./template-utils";
import { creatorDisplayName } from "./TemplateCreator";

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
  const lines = previewLines(template.detail);
  const showStatus = template.status && template.status !== "active";
  const showLanguage = template.language && template.language !== "other";

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
      className={`template-card${menuOpen ? " is-menu-open" : ""}`}
      onClick={() => onView(template)}
      onKeyDown={(e) => {
        if (e.key === "Enter") onView(template);
      }}
      role="button"
      tabIndex={0}
    >
      <header className="template-card-header">
        <div className="template-card-heading">
          <span className="template-card-kind">{labelOf(TEMPLATE_KIND_OPTIONS, template.kind)}</span>
          <h3 className="template-card-title" title={template.name}>{template.name}</h3>
        </div>
        <div className="template-card-actions" ref={menuRef} onClick={(e) => e.stopPropagation()}>
          <button
            type="button"
            className="template-card-menu-btn"
            aria-label="更多操作"
            aria-expanded={menuOpen}
            onClick={() => setMenuOpen((v) => !v)}
          >
            <MoreIcon />
          </button>
          {menuOpen ? (
            <div className="template-card-menu" role="menu">
              <button type="button" role="menuitem" onClick={() => { setMenuOpen(false); onView(template); }}>查看详情</button>
              {editable ? (
                <button type="button" role="menuitem" onClick={() => { setMenuOpen(false); onEdit(template); }}>编辑</button>
              ) : null}
              <button type="button" role="menuitem" onClick={() => { setMenuOpen(false); onCopy(template); }}>
                {editable ? "复制" : "复制到本工作区"}
              </button>
              {editable ? (
                <button type="button" role="menuitem" className="is-danger" onClick={() => { setMenuOpen(false); onDelete(template); }}>删除</button>
              ) : null}
            </div>
          ) : null}
        </div>
      </header>

      {lines.length > 0 ? (
        <pre className="template-card-preview" aria-hidden="true">{lines.join("\n")}</pre>
      ) : (
        <p className="template-card-preview is-empty">暂无内容</p>
      )}

      <footer className="template-card-footer">
        <span className="template-card-meta">
          {creatorDisplayName(template)} · {formatDate(template.created_at)}
        </span>
        <span className="template-card-tags">
          {showLanguage ? (
            <span className="template-card-tag">{labelOf(TEMPLATE_LANGUAGE_OPTIONS, template.language)}</span>
          ) : null}
          {template.shared ? (
            <span className="template-card-tag" title={template.workspace_name ?? "其他工作区"}>组织共享</span>
          ) : null}
          {showStatus ? (
            <span className={statusBadgeClass(template.status)}>
              {labelOf(TEMPLATE_STATUS_OPTIONS, template.status)}
            </span>
          ) : null}
        </span>
      </footer>
    </article>
  );
}

function MoreIcon() {
  return (
    <svg viewBox="0 0 24 24" width="16" height="16" fill="currentColor" aria-hidden="true">
      <circle cx="5" cy="12" r="1.6" />
      <circle cx="12" cy="12" r="1.6" />
      <circle cx="19" cy="12" r="1.6" />
    </svg>
  );
}
