"use client";

import { useState } from "react";
import type { ReleasePlanVersionDiffChangeVM } from "@sparkops/contracts/product";
import {
  describeJobSnapshot,
  describeWorkflowRefDiff,
  formatDiffValue,
  isJobSnapshot,
  isWorkflowRef,
  shouldUseStructuredDiff,
  type VersionDiffDisplayField,
  type VersionDiffDisplaySection,
} from "./release-plan-version-utils";

interface ReleasePlanVersionChangeDetailProps {
  change: ReleasePlanVersionDiffChangeVM;
  memberName?: (memberId?: string | null) => string;
}

function FieldGrid({ fields }: { fields: VersionDiffDisplayField[] }) {
  if (fields.length === 0) return null;
  return (
    <dl className="release-plan-version-field-grid">
      {fields.map((item) => (
        <div key={item.label} className="release-plan-version-field-row">
          <dt>{item.label}</dt>
          <dd className={item.mono ? "font-mono text-sm" : undefined}>{item.value}</dd>
        </div>
      ))}
    </dl>
  );
}

function SectionBlock({ section }: { section: VersionDiffDisplaySection }) {
  return (
    <div className="release-plan-version-detail-section">
      {section.title ? <p className="release-plan-version-detail-section-title">{section.title}</p> : null}
      <FieldGrid fields={section.fields} />
    </div>
  );
}

function RawJsonToggle({ value }: { value: unknown }) {
  const [open, setOpen] = useState(false);
  let raw = "";
  try {
    raw = JSON.stringify(value, null, 2);
  } catch {
    raw = String(value);
  }
  if (!raw || raw === "{}" || raw === "null") return null;
  return (
    <div className="release-plan-version-raw-wrap">
      <button
        type="button"
        className="btn-ghost text-xs px-0 release-plan-version-raw-toggle"
        onClick={() => setOpen((prev) => !prev)}
      >
        {open ? "收起原始数据" : "查看原始数据"}
      </button>
      {open ? <pre className="release-plan-version-diff-json">{raw}</pre> : null}
    </div>
  );
}

export function ReleasePlanVersionChangeDetail({
  change,
  memberName,
}: ReleasePlanVersionChangeDetailProps) {
  if (change.change_type === "job_added" || change.change_type === "job_removed") {
    const snapshot = change.after ?? change.before;
    const sections = describeJobSnapshot(snapshot, memberName);
    return (
      <div className="release-plan-version-detail stack-xs">
        {sections.map((section, index) => (
          <SectionBlock key={`${section.title ?? "main"}-${index}`} section={section} />
        ))}
        <RawJsonToggle value={snapshot} />
      </div>
    );
  }

  if (
    change.change_type === "field_changed" &&
    shouldUseStructuredDiff(change.before, change.after)
  ) {
    if (isJobSnapshot(change.before ?? change.after)) {
      const sections = describeJobSnapshot(change.after ?? change.before, memberName);
      return (
        <div className="release-plan-version-detail stack-xs">
          {sections.map((section, index) => (
            <SectionBlock key={`${section.title ?? "main"}-${index}`} section={section} />
          ))}
          <RawJsonToggle value={change.after ?? change.before} />
        </div>
      );
    }

    if (isWorkflowRef(change.before) || isWorkflowRef(change.after)) {
      const fields = describeWorkflowRefDiff(change.before, change.after);
      return (
        <div className="release-plan-version-detail stack-xs">
          <FieldGrid fields={fields} />
          <RawJsonToggle value={{ before: change.before, after: change.after }} />
        </div>
      );
    }
  }

  if (change.change_type === "field_changed" && (change.before !== undefined || change.after !== undefined)) {
    return (
      <div className="release-plan-version-detail">
        <div className="release-plan-version-before-after">
          <div>
            <span className="text-xs text-muted-foreground">变更前</span>
            <p>{formatDiffValue(change.before)}</p>
          </div>
          <div>
            <span className="text-xs text-muted-foreground">变更后</span>
            <p>{formatDiffValue(change.after)}</p>
          </div>
        </div>
      </div>
    );
  }

  return null;
}
