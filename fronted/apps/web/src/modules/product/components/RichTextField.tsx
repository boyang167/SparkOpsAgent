"use client";

import { useCallback, useEffect, useRef, useState } from "react";

interface RichTextFieldProps {
  value: string;
  onChange: (html: string) => void;
  onBlur?: (html: string) => void;
  placeholder?: string;
  readOnly?: boolean;
  className?: string;
}

function preventToolbarBlur(e: React.MouseEvent) {
  e.preventDefault();
}

function IconBold() {
  return (
    <svg viewBox="0 0 16 16" fill="currentColor" aria-hidden="true">
      <path d="M4 2.5h4.2a3.3 3.3 0 0 1 0 6.6H4V2.5zm0 7.1h4.8a3.1 3.1 0 0 1 0 6.2H4V9.6z" />
    </svg>
  );
}

function IconItalic() {
  return (
    <svg viewBox="0 0 16 16" fill="currentColor" aria-hidden="true">
      <path d="M9.5 2.5h-5v1.2h1.8L5.2 12.3H3.5v1.2h5v-1.2H6.7L9.8 3.7h1.7V2.5z" />
    </svg>
  );
}

function IconUnderline() {
  return (
    <svg viewBox="0 0 16 16" fill="currentColor" aria-hidden="true">
      <path d="M4 2.5v5.2a4 4 0 0 0 8 0V2.5h-1.4v5.2a2.6 2.6 0 0 1-5.2 0V2.5H4zm-1.2 10.8h9.4v1.2H2.8v-1.2z" />
    </svg>
  );
}

function IconBulletList() {
  return (
    <svg viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.5" aria-hidden="true">
      <path d="M6.5 4h7M6.5 8h7M6.5 12h7" strokeLinecap="round" />
      <circle cx="3.5" cy="4" r="1" fill="currentColor" stroke="none" />
      <circle cx="3.5" cy="8" r="1" fill="currentColor" stroke="none" />
      <circle cx="3.5" cy="12" r="1" fill="currentColor" stroke="none" />
    </svg>
  );
}

function IconNumberList() {
  return (
    <svg viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.5" aria-hidden="true">
      <path d="M6.5 4h7M6.5 8h7M6.5 12h7" strokeLinecap="round" />
      <path d="M2.8 3.2h.8v2.4H2.8M2.8 7.2h1.6M2.8 10.8h.9v.4H2.8" strokeLinecap="round" />
    </svg>
  );
}

function IconTable() {
  return (
    <svg viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.5" aria-hidden="true">
      <rect x="2.5" y="3" width="11" height="10" rx="1" />
      <path d="M2.5 7h11M6.5 3v10M10.5 3v10" />
    </svg>
  );
}

function IconAlignLeft() {
  return (
    <svg viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.5" aria-hidden="true">
      <path d="M2.5 4h11M2.5 8h7M2.5 12h9" strokeLinecap="round" />
    </svg>
  );
}

function IconAlignCenter() {
  return (
    <svg viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.5" aria-hidden="true">
      <path d="M2.5 4h11M4.5 8h7M3.5 12h9" strokeLinecap="round" />
    </svg>
  );
}

function IconAlignRight() {
  return (
    <svg viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.5" aria-hidden="true">
      <path d="M2.5 4h11M6.5 8h7M4.5 12h9" strokeLinecap="round" />
    </svg>
  );
}

function IconUndo() {
  return (
    <svg viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.5" aria-hidden="true">
      <path d="M3.5 6.5H10a3.5 3.5 0 1 1 0 7H8.5" strokeLinecap="round" strokeLinejoin="round" />
      <path d="M5.5 4.5 3.5 6.5l2 2" strokeLinecap="round" strokeLinejoin="round" />
    </svg>
  );
}

function IconRedo() {
  return (
    <svg viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.5" aria-hidden="true">
      <path d="M12.5 6.5H6a3.5 3.5 0 1 0 0 7H7.5" strokeLinecap="round" strokeLinejoin="round" />
      <path d="M10.5 4.5 12.5 6.5l-2 2" strokeLinecap="round" strokeLinejoin="round" />
    </svg>
  );
}

function IconChevronDown() {
  return (
    <svg viewBox="0 0 12 12" fill="none" stroke="currentColor" strokeWidth="1.5" aria-hidden="true">
      <path d="M3 4.5 6 7.5 9 4.5" strokeLinecap="round" strokeLinejoin="round" />
    </svg>
  );
}

function buildTableHtml(rows: number, cols: number) {
  let html = '<table class="rich-text-table"><tbody>';
  for (let r = 0; r < rows; r += 1) {
    html += "<tr>";
    for (let c = 0; c < cols; c += 1) {
      html += "<td>&nbsp;</td>";
    }
    html += "</tr>";
  }
  html += "</tbody></table><p><br></p>";
  return html;
}

const TABLE_PRESETS = [
  { label: "2 × 2", rows: 2, cols: 2 },
  { label: "3 × 3", rows: 3, cols: 3 },
  { label: "4 × 4", rows: 4, cols: 4 },
];

export function RichTextField({
  value,
  onChange,
  onBlur,
  placeholder = "请输入内容...",
  readOnly,
  className,
}: RichTextFieldProps) {
  const editorRef = useRef<HTMLDivElement>(null);
  const tableMenuRef = useRef<HTMLDivElement>(null);
  const latestHtmlRef = useRef(value);
  const onBlurRef = useRef(onBlur);
  const [tableMenuOpen, setTableMenuOpen] = useState(false);

  onBlurRef.current = onBlur;

  useEffect(() => {
    latestHtmlRef.current = value;
    const el = editorRef.current;
    if (!el || document.activeElement === el) return;
    if (el.innerHTML !== value) {
      el.innerHTML = value || "";
    }
  }, [value]);

  useEffect(() => {
    return () => {
      onBlurRef.current?.(latestHtmlRef.current);
    };
  }, []);

  useEffect(() => {
    if (!tableMenuOpen) return;
    const onPointerDown = (event: MouseEvent) => {
      if (!tableMenuRef.current?.contains(event.target as Node)) {
        setTableMenuOpen(false);
      }
    };
    document.addEventListener("mousedown", onPointerDown);
    return () => document.removeEventListener("mousedown", onPointerDown);
  }, [tableMenuOpen]);

  const syncChange = useCallback(() => {
    const html = editorRef.current?.innerHTML ?? "";
    latestHtmlRef.current = html;
    onChange(html);
  }, [onChange]);

  const exec = useCallback(
    (cmd: string, arg?: string) => {
      editorRef.current?.focus();
      document.execCommand(cmd, false, arg);
      syncChange();
    },
    [syncChange],
  );

  const insertTable = useCallback(
    (rows: number, cols: number) => {
      editorRef.current?.focus();
      document.execCommand("insertHTML", false, buildTableHtml(rows, cols));
      setTableMenuOpen(false);
      syncChange();
    },
    [syncChange],
  );

  return (
    <div className={className ? `rich-text-field ${className}` : "rich-text-field"}>
      {!readOnly ? (
        <div className="rich-text-toolbar">
          <div className="rich-text-toolbar-group">
            <button type="button" className="rich-text-tool rich-text-tool-heading" onMouseDown={preventToolbarBlur} onClick={() => exec("formatBlock", "h2")}>
              H2
            </button>
            <button type="button" className="rich-text-tool rich-text-tool-heading" onMouseDown={preventToolbarBlur} onClick={() => exec("formatBlock", "h3")}>
              H3
            </button>
            <button type="button" className="rich-text-tool rich-text-tool-heading" onMouseDown={preventToolbarBlur} onClick={() => exec("formatBlock", "h4")}>
              H4
            </button>
          </div>

          <div className="rich-text-toolbar-divider" aria-hidden="true" />

          <div className="rich-text-toolbar-group">
            <button type="button" className="rich-text-tool rich-text-tool-icon" title="粗体" onMouseDown={preventToolbarBlur} onClick={() => exec("bold")}>
              <IconBold />
            </button>
            <button type="button" className="rich-text-tool rich-text-tool-icon" title="斜体" onMouseDown={preventToolbarBlur} onClick={() => exec("italic")}>
              <IconItalic />
            </button>
            <button type="button" className="rich-text-tool rich-text-tool-icon" title="下划线" onMouseDown={preventToolbarBlur} onClick={() => exec("underline")}>
              <IconUnderline />
            </button>
          </div>

          <div className="rich-text-toolbar-divider" aria-hidden="true" />

          <div className="rich-text-toolbar-group">
            <button type="button" className="rich-text-tool rich-text-tool-icon" title="无序列表" onMouseDown={preventToolbarBlur} onClick={() => exec("insertUnorderedList")}>
              <IconBulletList />
            </button>
            <button type="button" className="rich-text-tool rich-text-tool-icon" title="有序列表" onMouseDown={preventToolbarBlur} onClick={() => exec("insertOrderedList")}>
              <IconNumberList />
            </button>
          </div>

          <div className="rich-text-toolbar-divider" aria-hidden="true" />

          <div className="rich-text-toolbar-group rich-text-table-menu" ref={tableMenuRef}>
            <button
              type="button"
              className={`rich-text-tool rich-text-tool-icon rich-text-tool-table${tableMenuOpen ? " is-active" : ""}`}
              title="插入表格"
              aria-expanded={tableMenuOpen}
              onMouseDown={preventToolbarBlur}
              onClick={() => setTableMenuOpen((open) => !open)}
            >
              <IconTable />
              <IconChevronDown />
            </button>
            {tableMenuOpen ? (
              <div className="rich-text-table-dropdown" role="menu">
                {TABLE_PRESETS.map((preset) => (
                  <button
                    key={preset.label}
                    type="button"
                    className="rich-text-table-option"
                    role="menuitem"
                    onMouseDown={preventToolbarBlur}
                    onClick={() => insertTable(preset.rows, preset.cols)}
                  >
                    {preset.label}
                  </button>
                ))}
              </div>
            ) : null}
          </div>

          <div className="rich-text-toolbar-divider" aria-hidden="true" />

          <div className="rich-text-toolbar-group">
            <button type="button" className="rich-text-tool rich-text-tool-icon" title="左对齐" onMouseDown={preventToolbarBlur} onClick={() => exec("justifyLeft")}>
              <IconAlignLeft />
            </button>
            <button type="button" className="rich-text-tool rich-text-tool-icon" title="居中对齐" onMouseDown={preventToolbarBlur} onClick={() => exec("justifyCenter")}>
              <IconAlignCenter />
            </button>
            <button type="button" className="rich-text-tool rich-text-tool-icon" title="右对齐" onMouseDown={preventToolbarBlur} onClick={() => exec("justifyRight")}>
              <IconAlignRight />
            </button>
          </div>

          <div className="rich-text-toolbar-divider" aria-hidden="true" />

          <div className="rich-text-toolbar-group">
            <button type="button" className="rich-text-tool rich-text-tool-icon" title="撤销" onMouseDown={preventToolbarBlur} onClick={() => exec("undo")}>
              <IconUndo />
            </button>
            <button type="button" className="rich-text-tool rich-text-tool-icon" title="重做" onMouseDown={preventToolbarBlur} onClick={() => exec("redo")}>
              <IconRedo />
            </button>
          </div>
        </div>
      ) : null}
      <div
        ref={editorRef}
        className="rich-text-editor input text-sm prose prose-sm max-w-none dark:prose-invert"
        contentEditable={!readOnly}
        suppressContentEditableWarning
        data-placeholder={placeholder}
        onInput={syncChange}
        onBlur={() => {
          const html = editorRef.current?.innerHTML ?? latestHtmlRef.current;
          latestHtmlRef.current = html;
          onChange(html);
          onBlurRef.current?.(html);
        }}
      />
    </div>
  );
}
