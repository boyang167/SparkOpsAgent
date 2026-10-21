"use client";

import { useEffect, useMemo, useRef, useState } from "react";
import { ApiErrorBanner } from "./ApiErrorBanner";
import { parseYamlEditorDocument, stringifyYamlEditorDocument } from "./yaml-document";

type YamlEditorProps = {
  title?: string;
  description?: string;
  value: unknown;
  canSave: boolean;
  saving?: boolean;
  confirmLabel?: string;
  cancelLabel?: string;
  onSave: (next: unknown) => Promise<void>;
  onClose: () => void;
};

function stripManagedFields(value: unknown): unknown {
  if (!value || typeof value !== "object" || Array.isArray(value)) return value;
  const obj = { ...(value as Record<string, unknown>) };
  const metadata = obj.metadata;
  if (metadata && typeof metadata === "object" && !Array.isArray(metadata)) {
    const nextMeta = { ...(metadata as Record<string, unknown>) };
    delete nextMeta.managedFields;
    obj.metadata = nextMeta;
  }
  delete obj.status;
  return obj;
}

export function YamlEditor({
  title,
  description,
  value,
  canSave,
  saving = false,
  confirmLabel = "确定",
  cancelLabel = "取消",
  onSave,
  onClose,
}: YamlEditorProps) {
  const initialText = useMemo(() => stringifyYamlEditorDocument(stripManagedFields(value)), [value]);
  const [text, setText] = useState(initialText);
  const [error, setError] = useState<string | null>(null);
  const gutterRef = useRef<HTMLDivElement>(null);
  const lines = text.split("\n");

  useEffect(() => {
    setText(initialText);
    setError(null);
  }, [initialText]);

  async function save() {
    setError(null);
    try {
      await onSave(parseYamlEditorDocument(text));
    } catch (err) {
      setError(err instanceof Error ? err.message : "保存失败");
    }
  }

  return (
    <div className="stack-md">
      {title ? (
        <div>
          <h3 className="text-lg font-semibold">{title}</h3>
          {description ? <p className="text-sm text-muted mt-1">{description}</p> : null}
        </div>
      ) : null}
      <div className="k8s-yaml-editor" data-theme="light">
        <div className="k8s-yaml-gutter" aria-hidden ref={gutterRef}>
          {lines.map((_, index) => (
            <div key={index}>{index + 1}</div>
          ))}
        </div>
        <textarea
          className="k8s-yaml-textarea"
          value={text}
          spellCheck={false}
          onChange={(e) => setText(e.target.value)}
          onScroll={(e) => {
            if (gutterRef.current) gutterRef.current.scrollTop = e.currentTarget.scrollTop;
          }}
        />
      </div>
      <ApiErrorBanner error={error} />
      <div className="flex justify-end gap-2">
        <button type="button" className="btn-secondary" disabled={saving} onClick={onClose}>
          {cancelLabel}
        </button>
        {canSave ? (
          <button type="button" className="btn" disabled={saving} onClick={() => void save()}>
            {saving ? "保存中…" : confirmLabel}
          </button>
        ) : null}
      </div>
    </div>
  );
}
