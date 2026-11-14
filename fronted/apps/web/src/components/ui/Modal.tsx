"use client";

import { useEffect, useId, useRef } from "react";
import { createPortal } from "react-dom";
import { useModalFocus } from "./modal-focus";

interface ModalProps {
  open: boolean;
  onClose: () => void;
  title: string;
  description?: string;
  size?: "sm" | "md" | "lg" | "xl" | "full";
  /** Higher stacking + capture Escape so nested dialogs do not close the parent. */
  nested?: boolean;
  /** Above drawers. Use when this dialog is opened from a drawer. */
  priority?: boolean;
  panelClassName?: string;
  headerExtra?: React.ReactNode;
  children: React.ReactNode;
  footer?: React.ReactNode;
}

export function Modal({
  open,
  onClose,
  title,
  description,
  size = "md",
  nested = false,
  priority = false,
  panelClassName,
  headerExtra,
  children,
  footer,
}: ModalProps) {
  const titleId = useId();
  const descId = useId();
  const onCloseRef = useRef(onClose);
  const panelRef = useModalFocus<HTMLDivElement>(open);
  onCloseRef.current = onClose;

  useEffect(() => {
    if (!open) return;

    function onKeyDown(e: KeyboardEvent) {
      if (e.key !== "Escape") return;
      if (nested) e.stopPropagation();
      onCloseRef.current();
    }

    document.addEventListener("keydown", onKeyDown, nested);
    const prevOverflow = document.body.style.overflow;
    document.body.style.overflow = "hidden";

    return () => {
      document.removeEventListener("keydown", onKeyDown, nested);
      document.body.style.overflow = prevOverflow;
    };
  }, [open, nested]);

  if (!open) return null;

  const modal = (
    <div
      className={`modal-root${nested ? " modal-nested" : ""}${priority ? " modal-priority" : ""}`}
      role="presentation"
    >
      <button
        type="button"
        className="modal-backdrop"
        aria-label="关闭对话框"
        onClick={() => onCloseRef.current()}
      />
      <div
        ref={panelRef}
        className={`modal-panel modal-panel-${size}${panelClassName ? ` ${panelClassName}` : ""}`}
        role="dialog"
        tabIndex={-1}
        aria-modal="true"
        aria-labelledby={titleId}
        aria-describedby={description ? descId : undefined}
      >
        <header className="modal-header">
          <div className="modal-header-copy">
            <h2 id={titleId} className="modal-title">
              {title}
            </h2>
            {description ? (
              <p id={descId} className="modal-description">
                {description}
              </p>
            ) : null}
          </div>
          <div className="modal-header-actions">
            {headerExtra}
            <button
              type="button"
              className="modal-close"
              onClick={() => onCloseRef.current()}
              aria-label="关闭"
            >
              <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
                <path d="M18 6 6 18M6 6l12 12" strokeLinecap="round" />
              </svg>
            </button>
          </div>
        </header>
        <div className="modal-body">{children}</div>
        {footer ? <footer className="modal-footer">{footer}</footer> : null}
      </div>
    </div>
  );

  return typeof document !== "undefined" ? createPortal(modal, document.body) : null;
}
