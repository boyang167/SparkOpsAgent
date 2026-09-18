"use client";

import { useEffect, useId, useRef, useState } from "react";
import { createPortal } from "react-dom";

interface SelectOrManualFieldProps {
  label: string;
  value: string;
  options: string[];
  loading?: boolean;
  hint?: string | null;
  placeholder?: string;
  emptyLabel?: string;
  pinValue?: boolean;
  onChange: (value: string) => void;
}

export function SelectOrManualField({
  label,
  value,
  options,
  loading = false,
  hint,
  placeholder,
  emptyLabel,
  pinValue = false,
  onChange,
}: SelectOrManualFieldProps) {
  const [mode, setMode] = useState<"select" | "manual">("select");
  const [open, setOpen] = useState(false);
  const [menuStyle, setMenuStyle] = useState<{ top: number; left: number; width: number; maxHeight: number } | null>(
    null,
  );
  const rootRef = useRef<HTMLDivElement>(null);
  const listId = useId();
  const names = new Set(options.filter(Boolean));
  if (pinValue && value.trim()) names.add(value.trim());
  const items = [...names];

  function placeMenu() {
    const rect = rootRef.current?.getBoundingClientRect();
    if (!rect) return;
    const spaceBelow = window.innerHeight - rect.bottom - 12;
    const spaceAbove = rect.top - 12;
    const openUp = spaceBelow < 160 && spaceAbove > spaceBelow;
    const maxHeight = Math.max(96, Math.min(192, openUp ? spaceAbove : spaceBelow));
    setMenuStyle({
      top: openUp ? Math.max(8, rect.top - maxHeight - 4) : rect.bottom + 4,
      left: rect.left,
      width: rect.width,
      maxHeight,
    });
  }

  useEffect(() => {
    if (!open) return;
    function onPointer(event: MouseEvent) {
      const target = event.target;
      if (target instanceof Node && rootRef.current?.contains(target)) return;
      if (target instanceof Element && target.closest(`[data-menu="${listId}"]`)) return;
      setOpen(false);
    }
    function onLayout() {
      placeMenu();
    }
    document.addEventListener("mousedown", onPointer);
    window.addEventListener("resize", onLayout);
    window.addEventListener("scroll", onLayout, true);
    return () => {
      document.removeEventListener("mousedown", onPointer);
      window.removeEventListener("resize", onLayout);
      window.removeEventListener("scroll", onLayout, true);
    };
  }, [open, listId]);

  return (
    <div className="field">
      <span className="pwf-select-label">
        <span className="text-xs text-muted">{label}</span>
        <button
          type="button"
          className="pwf-mode-link"
          onClick={() => {
            setOpen(false);
            setMode((current) => (current === "select" ? "manual" : "select"));
          }}
        >
          {mode === "select" ? "手动输入" : "下拉选择"}
        </button>
      </span>
      {mode === "select" ? (
        <div className="pwf-combo" ref={rootRef}>
          <button
            type="button"
            className="input pwf-combo-trigger"
            aria-label={label}
            aria-expanded={open}
            aria-controls={listId}
            disabled={loading}
            onClick={() => {
              if (open) {
                setOpen(false);
                return;
              }
              placeMenu();
              setOpen(true);
            }}
          >
            <span>{value || emptyLabel || (loading ? "加载中…" : "请选择")}</span>
            <span aria-hidden>{open ? "▴" : "▾"}</span>
          </button>
          {open && menuStyle
            ? createPortal(
                <ul
                  id={listId}
                  data-menu={listId}
                  className="pwf-combo-menu"
                  role="listbox"
                  style={menuStyle}
                >
                  {items.length === 0 ? (
                    <li className="pwf-combo-empty">{emptyLabel || "暂无选项"}</li>
                  ) : (
                    items.map((item) => (
                      <li key={item}>
                        <button
                          type="button"
                          role="option"
                          aria-selected={item === value}
                          className={item === value ? "is-active" : undefined}
                          onClick={() => {
                            onChange(item);
                            setOpen(false);
                          }}
                        >
                          {item}
                        </button>
                      </li>
                    ))
                  )}
                </ul>,
                document.body,
              )
            : null}
        </div>
      ) : (
        <input
          className="input"
          aria-label={label}
          placeholder={placeholder}
          value={value}
          onChange={(e) => onChange(e.target.value)}
        />
      )}
      {hint ? <p className="text-xs text-muted">{hint}</p> : null}
    </div>
  );
}
