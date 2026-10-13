"use client";

import { useEffect, useRef, useState, type ReactNode } from "react";

export interface ActionMenuItem {
  label: string;
  danger?: boolean;
  disabled?: boolean;
  title?: string;
  separatorBefore?: boolean;
  onSelect: () => void;
}

interface ActionMenuProps {
  trigger: ReactNode;
  ariaLabel: string;
  items: (ActionMenuItem | null | false)[];
  triggerClassName?: string;
  disabled?: boolean;
}

export function ActionMenu({ trigger, ariaLabel, items, triggerClassName = "action-menu-trigger", disabled }: ActionMenuProps) {
  const [open, setOpen] = useState(false);
  const rootRef = useRef<HTMLDivElement>(null);
  const visible = items.filter((item): item is ActionMenuItem => Boolean(item));

  useEffect(() => {
    if (!open) return;
    const onPointerDown = (event: MouseEvent) => {
      if (!rootRef.current?.contains(event.target as Node)) setOpen(false);
    };
    const onKeyDown = (event: KeyboardEvent) => {
      if (event.key === "Escape") setOpen(false);
    };
    document.addEventListener("mousedown", onPointerDown);
    document.addEventListener("keydown", onKeyDown);
    return () => {
      document.removeEventListener("mousedown", onPointerDown);
      document.removeEventListener("keydown", onKeyDown);
    };
  }, [open]);

  if (visible.length === 0) return null;

  return (
    <div className={`action-menu${open ? " is-open" : ""}`} ref={rootRef}>
      <button
        type="button"
        className={triggerClassName}
        aria-label={ariaLabel}
        aria-haspopup="menu"
        aria-expanded={open}
        disabled={disabled}
        onClick={() => setOpen((value) => !value)}
      >
        {trigger}
      </button>
      {open ? (
        <div className="action-menu-list" role="menu">
          {visible.map((item) => (
            <div key={item.label} role="none">
              {item.separatorBefore ? <div className="action-menu-sep" role="separator" /> : null}
              <button
                type="button"
                role="menuitem"
                className={`action-menu-item${item.danger ? " is-danger" : ""}`}
                disabled={item.disabled}
                title={item.title}
                onClick={() => {
                  setOpen(false);
                  item.onSelect();
                }}
              >
                {item.label}
              </button>
            </div>
          ))}
        </div>
      ) : null}
    </div>
  );
}
