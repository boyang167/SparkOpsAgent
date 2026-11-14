"use client";

import { createElement, useEffect, useId, useRef, useState } from "react";
import { createPortal } from "react-dom";
import { useModalFocus } from "./modal-focus";

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
  const panelRef = useModalFocus<HTMLDivElement>(open && mounted);

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
    createElement(
      "div",
      { className: "drawer-root", role: "presentation" },
      createElement("button", {
        type: "button",
        className: "drawer-backdrop",
        "aria-label": "关闭抽屉",
        onClick: () => onCloseRef.current(),
      }),
      createElement(
        "div",
        {
          ref: panelRef,
          tabIndex: -1,
          className: panelClassName ? `drawer-panel ${panelClassName}` : "drawer-panel",
          role: "dialog",
          "aria-modal": true,
          "aria-labelledby": titleId,
          "aria-describedby": description ? descId : undefined,
        },
        createElement(
          "header",
          { className: "drawer-header" },
          createElement(
            "div",
            null,
            createElement("h2", { id: titleId, className: "drawer-title" }, title),
            description
              ? createElement("p", { id: descId, className: "drawer-description" }, description)
              : null,
          ),
          createElement(
            "button",
            {
              type: "button",
              className: "drawer-close",
              onClick: () => onCloseRef.current(),
              "aria-label": "关闭",
            },
            createElement(
              "svg",
              {
                viewBox: "0 0 24 24",
                fill: "none",
                stroke: "currentColor",
                strokeWidth: "2",
              },
              createElement("path", {
                d: "M18 6 6 18M6 6l12 12",
                strokeLinecap: "round",
              }),
            ),
          ),
        ),
        tabs ? createElement("div", { className: "drawer-tabs" }, tabs) : null,
        createElement("div", { className: "drawer-body" }, children),
        footer ? createElement("footer", { className: "drawer-footer" }, footer) : null,
      ),
    ),
    document.body,
  );
}
