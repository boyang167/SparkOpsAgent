"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";
import { useEffect, useRef, useState } from "react";
import {
  buildConsoleOptions,
  CONSOLE_LABELS,
  resolveConsoleKind,
} from "@/components/layout/nav-tree";

interface ConsoleSwitcherProps {
  isSuperAdmin: boolean;
  canManageAdmin: boolean;
}

export function ConsoleSwitcher({ isSuperAdmin, canManageAdmin }: ConsoleSwitcherProps) {
  const pathname = usePathname();
  const current = resolveConsoleKind(pathname);
  const options = buildConsoleOptions(isSuperAdmin, canManageAdmin);
  const [open, setOpen] = useState(false);
  const rootRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    function onPointerDown(e: MouseEvent) {
      if (!rootRef.current?.contains(e.target as Node)) {
        setOpen(false);
      }
    }
    document.addEventListener("mousedown", onPointerDown);
    return () => document.removeEventListener("mousedown", onPointerDown);
  }, []);

  if (options.length <= 1) {
    return <span className="console-switcher-label">{CONSOLE_LABELS[current]}</span>;
  }

  const active = options.find((item) => item.kind === current) ?? options[0];

  return (
    <div className="console-switcher" ref={rootRef}>
      <button
        type="button"
        className="console-switcher-trigger"
        aria-haspopup="listbox"
        aria-expanded={open}
        onClick={() => setOpen((value) => !value)}
      >
        <span>{active.label}</span>
        <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" aria-hidden>
          <path d="M6 9l6 6 6-6" strokeLinecap="round" strokeLinejoin="round" />
        </svg>
      </button>
      {open ? (
        <div className="console-switcher-menu" role="listbox">
          {options.map((option) => (
            <Link
              key={option.kind}
              href={option.href}
              className={`console-switcher-item ${option.kind === current ? "console-switcher-item-active" : ""}`}
              role="option"
              aria-selected={option.kind === current}
              onClick={() => setOpen(false)}
            >
              {option.label}
            </Link>
          ))}
        </div>
      ) : null}
    </div>
  );
}
