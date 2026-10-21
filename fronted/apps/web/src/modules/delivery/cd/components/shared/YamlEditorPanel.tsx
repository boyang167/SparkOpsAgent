"use client";

import { useMemo } from "react";

interface YamlEditorPanelProps {
  value: string;
  onChange: (value: string) => void;
  readOnly?: boolean;
  error?: string | null;
  className?: string;
}

export function YamlEditorPanel({
  value,
  onChange,
  readOnly = false,
  error,
  className,
}: YamlEditorPanelProps) {
  const lines = useMemo(() => value.split("\n"), [value]);

  return (
    <div className={className ? `cd-yaml-panel ${className}` : "cd-yaml-panel"}>
      {error ? <p className="cd-yaml-error">{error}</p> : null}
      <div className="k8s-yaml-editor">
        <div className="k8s-yaml-gutter" aria-hidden="true">
          {lines.map((_, index) => (
            <div key={index}>{index + 1}</div>
          ))}
        </div>
        <textarea
          className="k8s-yaml-textarea"
          value={value}
          readOnly={readOnly}
          spellCheck={false}
          onChange={(event) => onChange(event.target.value)}
        />
      </div>
    </div>
  );
}
