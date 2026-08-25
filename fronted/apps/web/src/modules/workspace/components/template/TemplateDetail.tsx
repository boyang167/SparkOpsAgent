"use client";

import { useState } from "react";
import type { TemplateVM } from "@sparkops/contracts/product";
import {
  formatDateTime,
  labelOf,
  statusBadgeClass,
  TEMPLATE_KIND_OPTIONS,
  TEMPLATE_LANGUAGE_OPTIONS,
  TEMPLATE_STATUS_OPTIONS,
  templateVisibilityLabel,
} from "./template-utils";
import { TemplateCreator } from "./TemplateCreator";

interface TemplateDetailProps {
  template: TemplateVM;
  onCopyContent: (text: string) => void;
  onDownload: (template: TemplateVM) => void;
}

type DetailTab = "content" | "info" | "stats";

export function TemplateDetail({ template, onCopyContent, onDownload }: TemplateDetailProps) {
  const [tab, setTab] = useState<DetailTab>("content");

  return (
    <div className="template-detail">
      <div className="template-detail-header">
        <div className="template-detail-title">
          <DocumentIcon />
          <h3>{template.name}</h3>
        </div>
        <div className="template-detail-meta">
          <span className={statusBadgeClass(template.status)}>
            {labelOf(TEMPLATE_STATUS_OPTIONS, template.status)}
          </span>
          <span className="badge badge-primary">{labelOf(TEMPLATE_KIND_OPTIONS, template.kind)}</span>
          <span className="badge badge-neutral">{labelOf(TEMPLATE_LANGUAGE_OPTIONS, template.language)}</span>
        </div>
      </div>

      <div className="template-tabs">
        <div className="template-tabs-nav">
          <button type="button" className={tab === "content" ? "is-active" : ""} onClick={() => setTab("content")}>模板内容</button>
          <button type="button" className={tab === "info" ? "is-active" : ""} onClick={() => setTab("info")}>基本信息</button>
          <button type="button" className={tab === "stats" ? "is-active" : ""} onClick={() => setTab("stats")}>使用统计</button>
        </div>

        <div className="template-tabs-body">
          {tab === "content" ? (
            <div className="template-content-section">
              <div className="template-content-toolbar">
                <h4>模板内容</h4>
                <div className="template-content-actions">
                  <button type="button" className="btn-secondary text-sm" onClick={() => onCopyContent(template.detail ?? "")}>复制内容</button>
                  <button type="button" className="btn-secondary text-sm" onClick={() => onDownload(template)}>下载文件</button>
                </div>
              </div>
              <textarea className="input template-content-readonly" readOnly value={template.detail ?? ""} />
            </div>
          ) : null}

          {tab === "info" ? (
            <dl className="template-info-grid">
              <div><dt>模板名称</dt><dd>{template.name}</dd></div>
              <div><dt>模板分类</dt><dd><span className="badge badge-primary">{labelOf(TEMPLATE_KIND_OPTIONS, template.kind)}</span></dd></div>
              <div><dt>开发语言</dt><dd><span className="badge badge-neutral">{labelOf(TEMPLATE_LANGUAGE_OPTIONS, template.language)}</span></dd></div>
              <div><dt>模板状态</dt><dd><span className={statusBadgeClass(template.status)}>{labelOf(TEMPLATE_STATUS_OPTIONS, template.status)}</span></dd></div>
              <div><dt>可见性</dt><dd>{templateVisibilityLabel(template)}</dd></div>
              {template.workspace_name ? (
                <div><dt>来源工作区</dt><dd>{template.workspace_name}</dd></div>
              ) : null}
              <div><dt>创建时间</dt><dd>{formatDateTime(template.created_at)}</dd></div>
              <div><dt>更新时间</dt><dd>{formatDateTime(template.updated_at)}</dd></div>
              <div><dt>创建者</dt><dd><TemplateCreator template={template} /></dd></div>
            </dl>
          ) : null}

          {tab === "stats" ? (
            <div className="template-stats-grid">
              <div className="template-stat-card"><div className="template-stat-value">0</div><div className="template-stat-label">使用次数</div></div>
              <div className="template-stat-card"><div className="template-stat-value">0</div><div className="template-stat-label">收藏次数</div></div>
              <div className="template-stat-card"><div className="template-stat-value">0</div><div className="template-stat-label">评分</div></div>
            </div>
          ) : null}
        </div>
      </div>
    </div>
  );
}

function DocumentIcon() {
  return (
    <svg className="template-doc-icon-lg" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
      <path d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8z" />
      <path d="M14 2v6h6M16 13H8M16 17H8M10 9H8" strokeLinecap="round" />
    </svg>
  );
}
