"use client";

import Link from "next/link";
import { useLayoutEffect, useState, type CSSProperties } from "react";
import { createPortal } from "react-dom";
import { IconMore, IconPencil, IconRun } from "./WorkflowListIcons";

interface WorkflowRowMenuProps {
  open: boolean;
  detailHref: string;
  editHref: string;
  onToggle: () => void;
  onClose: () => void;
}

const MENU_WIDTH = 196;
const MENU_HEIGHT = 96;
const GAP = 6;

function placeMenu(anchor: HTMLElement): CSSProperties {
  const rect = anchor.getBoundingClientRect();
  const margin = 8;
  const spaceBelow = window.innerHeight - rect.bottom - margin;
  const openUp = spaceBelow < MENU_HEIGHT && rect.top > MENU_HEIGHT;
  const right = Math.max(margin, window.innerWidth - rect.right);
  if (openUp) {
    return {
      position: "fixed",
      right,
      bottom: window.innerHeight - rect.top + GAP,
      width: MENU_WIDTH,
      zIndex: 80,
    };
  }
  return {
    position: "fixed",
    right,
    top: Math.min(rect.bottom + GAP, window.innerHeight - MENU_HEIGHT - margin),
    width: MENU_WIDTH,
    zIndex: 80,
  };
}

export function WorkflowRowMenu({
  open,
  detailHref,
  editHref,
  onToggle,
  onClose,
}: WorkflowRowMenuProps) {
  const [anchor, setAnchor] = useState<HTMLButtonElement | null>(null);
  const [style, setStyle] = useState<CSSProperties | null>(null);

  useLayoutEffect(() => {
    if (!open || !anchor) {
      setStyle(null);
      return;
    }
    const update = () => setStyle(placeMenu(anchor));
    update();
    window.addEventListener("resize", update);
    window.addEventListener("scroll", update, true);
    const onKey = (event: KeyboardEvent) => {
      if (event.key === "Escape") onClose();
    };
    window.addEventListener("keydown", onKey);
    return () => {
      window.removeEventListener("resize", update);
      window.removeEventListener("scroll", update, true);
      window.removeEventListener("keydown", onKey);
    };
  }, [open, anchor, onClose]);

  return (
    <div className="wf-list-menu-wrap">
      <button
        ref={setAnchor}
        type="button"
        className={`wf-list-icon-btn${open ? " is-active" : ""}`}
        aria-haspopup="menu"
        aria-expanded={open}
        title="更多操作"
        onClick={(event) => {
          event.stopPropagation();
          onToggle();
        }}
      >
        <IconMore />
      </button>
      {open && style
        ? createPortal(
            <div
              className="wf-list-menu"
              role="menu"
              style={style}
              onClick={(event) => event.stopPropagation()}
            >
              <Link href={detailHref} className="wf-list-menu-item" role="menuitem" onClick={onClose}>
                <span className="wf-list-menu-item-icon">
                  <IconRun />
                </span>
                执行记录
              </Link>
              <Link href={editHref} className="wf-list-menu-item" role="menuitem" onClick={onClose}>
                <span className="wf-list-menu-item-icon">
                  <IconPencil />
                </span>
                编辑编排
              </Link>
            </div>,
            document.body,
          )
        : null}
    </div>
  );
}
