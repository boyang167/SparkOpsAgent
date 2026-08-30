"use client";

import { useMemo, useState } from "react";
import { ApiErrorBanner } from "./ApiErrorBanner";
import { parseYamlEditorDocument, stringifyYamlEditorDocument } from "./yaml-document";

type YamlEditorProps = {
  title: string;
  value: unknown;
  canSave: boolean;
  saving?: boolean;
  onSave: (next: unknown) => Promise<void>;
  onClose: () => void;
};

export function YamlEditor({ title, value, canSave, saving = false, onSave, onClose }: YamlEditorProps) {
  const initialText = useMemo(() => stringifyYamlEditorDocument(value), [value]);
  const [text, setText] = useState(initialText);
  const [error, setError] = useState<string | null>(null);

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
      <div>
        <h3 className="text-lg font-semibold">{title}</h3>
        <p className="text-sm text-muted mt-1">YAML/JSON（支持 YAML 与 JSON，保存前会拒绝非对象根和危险原型键）。</p>
      </div>
      <textarea
        className="input input-mono min-h-96 text-xs"
        value={text}
        spellCheck={false}
        onChange={(e) => setText(e.target.value)}
      />
      <ApiErrorBanner error={error} />
      <div className="flex justify-end gap-2">
        <button type="button" className="btn-secondary" onClick={onClose}>
          关闭
        </button>
        {canSave ? (
          <button type="button" className="btn" disabled={saving} onClick={() => void save()}>
            {saving ? "保存中…" : "保存"}
          </button>
        ) : null}
      </div>
    </div>
  );
}
