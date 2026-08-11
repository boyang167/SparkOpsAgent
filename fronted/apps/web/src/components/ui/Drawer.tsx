"use client";

import { useEffect, useId, useRef, useState } from "react";
import { createPortal } from "react-dom";

interface DrawerProps {
  open: boolean;
  onClose: () => void;
  title: string;
  description?: string;
  tabs?: React.ReactNode;
  children: React.ReactNode;
  footer?: React.ReactNode;
  panelClassName?: string;
}

export function Drawer({
  open,
  onClose,
  title,
  description,
  tabs,
  children,
  footer,
  panelClassName,
}: DrawerProps) {
  const titleId = useId();
  const descId = useId();
  const onCloseRef = useRef(onClose);
  onCloseRef.current = onClose;
  const [mounted, setMounted] = useState(false);

  useEffect(() => {
    setMounted(true);
  }, []);

  useEffect(() => {
    if (!open) return;

    function onKeyDown(e: KeyboardEvent) {
      if (e.key === "Escape") onCloseRef.current();
    }

    document.addEventListener("keydown", onKeyDown);
    const prevOverflow = document.body.style.overflow;
    document.body.style.overflow = "hidden";

    return () => {
      document.removeEventListener("keydown", onKeyDown);
      document.body.style.overflow = prevOverflow;
    };
  }, [open]);

  if (!open || !mounted) return null;

  return createPortal(
    <div className="drawer-root" role="presentation">
      <button
        type="button"
        className="drawer-backdrop"
        aria-label="关闭抽屉"
        onClick={() => onCloseRef.current()}
      />
      <div
        className={panelClassName ? `drawer-panel ${panelClassName}` : "drawer-panel"}
        role="dialog"
        aria-modal="true"
        aria-labelledby={titleId}
        aria-describedby={description ? descId : undefined}
      >
        <header className="drawer-header">
          <div>
            <h2 id={titleId} className="drawer-title">
              {title}
            </h2>
            {description ? (
              <p id={descId} className="drawer-description">
                {description}
              </p>
            ) : null}
          </div>
          <button
            type="button"
            className="drawer-close"
            onClick={() => onCloseRef.current()}
            aria-label="关闭"
          >
            <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
              <path d="M18 6 6 18M6 6l12 12" strokeLinecap="round" />
            </svg>
          </button>
        </header>
        {tabs ? <div className="drawer-tabs">{tabs}</div> : null}
        <div className="drawer-body">{children}</div>
        {footer ? <footer className="drawer-footer">{footer}</footer> : null}
      </div>
    </div>,
    document.body,
  );
}
