"use client";

import { useCallback, useEffect, useRef } from "react";

interface RichTextFieldProps {
  value: string;
  onChange: (html: string) => void;
  placeholder?: string;
  readOnly?: boolean;
  className?: string;
}

export function RichTextField({ value, onChange, placeholder, readOnly, className }: RichTextFieldProps) {
  const editorRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    const el = editorRef.current;
    if (!el || document.activeElement === el) return;
    if (el.innerHTML !== value) {
      el.innerHTML = value || "";
    }
  }, [value]);

  const exec = useCallback((cmd: string, arg?: string) => {
    document.execCommand(cmd, false, arg);
    const html = editorRef.current?.innerHTML ?? "";
    onChange(html);
  }, [onChange]);

  return (
    <div className={className}>
      {!readOnly ? (
        <div className="flex flex-wrap gap-1 mb-1">
          <button type="button" className="btn-secondary text-xs px-2 py-0.5" onMouseDown={(e) => e.preventDefault()} onClick={() => exec("bold")}>
            粗体
          </button>
          <button type="button" className="btn-secondary text-xs px-2 py-0.5" onMouseDown={(e) => e.preventDefault()} onClick={() => exec("italic")}>
            斜体
          </button>
          <button type="button" className="btn-secondary text-xs px-2 py-0.5" onMouseDown={(e) => e.preventDefault()} onClick={() => exec("insertUnorderedList")}>
            列表
          </button>
          <button type="button" className="btn-secondary text-xs px-2 py-0.5" onMouseDown={(e) => e.preventDefault()} onClick={() => exec("insertOrderedList")}>
            编号
          </button>
        </div>
      ) : null}
      <div
        ref={editorRef}
        className="input min-h-[80px] text-sm prose prose-sm max-w-none dark:prose-invert"
        contentEditable={!readOnly}
        suppressContentEditableWarning
        data-placeholder={placeholder}
        onInput={() => onChange(editorRef.current?.innerHTML ?? "")}
        onBlur={() => onChange(editorRef.current?.innerHTML ?? "")}
      />
    </div>
  );
}
