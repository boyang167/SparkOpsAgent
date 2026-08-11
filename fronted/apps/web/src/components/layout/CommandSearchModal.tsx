"use client";

import { useEffect, useMemo, useRef, useState } from "react";
import { createPortal } from "react-dom";
import { useRouter } from "next/navigation";
import { buildNavSearchItems, filterNavSearchItems } from "./nav-search";

interface CommandSearchModalProps {
  open: boolean;
  onClose: () => void;
}

export function CommandSearchModal({ open, onClose }: CommandSearchModalProps) {
  const router = useRouter();
  const inputRef = useRef<HTMLInputElement>(null);
  const [keyword, setKeyword] = useState("");
  const [activeIndex, setActiveIndex] = useState(0);
  const [mounted, setMounted] = useState(false);

  useEffect(() => {
    setMounted(true);
  }, []);

  const items = useMemo(() => filterNavSearchItems(buildNavSearchItems(), keyword), [keyword]);

  useEffect(() => {
    if (!open) return;
    setKeyword("");
    setActiveIndex(0);
    window.setTimeout(() => inputRef.current?.focus(), 0);
  }, [open]);

  useEffect(() => {
    if (!open) return;
    function onKeyDown(e: KeyboardEvent) {
      if (e.key === "Escape") onClose();
    }
    document.addEventListener("keydown", onKeyDown);
    return () => document.removeEventListener("keydown", onKeyDown);
  }, [open, onClose]);

  useEffect(() => {
    setActiveIndex(0);
  }, [keyword]);

  if (!open || !mounted) return null;

  function navigate(href: string) {
    onClose();
    router.push(href);
  }

  function onKeyDownInput(e: React.KeyboardEvent<HTMLInputElement>) {
    if (e.key === "ArrowDown") {
      e.preventDefault();
      setActiveIndex((prev) => Math.min(items.length - 1, prev + 1));
    } else if (e.key === "ArrowUp") {
      e.preventDefault();
      setActiveIndex((prev) => Math.max(0, prev - 1));
    } else if (e.key === "Enter" && items[activeIndex]) {
      e.preventDefault();
      navigate(items[activeIndex].href);
    }
  }

  return createPortal(
    <div className="command-search-root" role="presentation">
      <button type="button" className="command-search-backdrop" aria-label="关闭搜索" onClick={onClose} />
      <div className="command-search-panel" role="dialog" aria-modal="true" aria-label="搜索">
        <div className="command-search-input-wrap">
          <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" aria-hidden>
            <circle cx="11" cy="11" r="7" />
            <path d="M20 20l-3-3" strokeLinecap="round" />
          </svg>
          <input
            ref={inputRef}
            className="command-search-input"
            placeholder="搜索菜单、页面路径…"
            value={keyword}
            onChange={(e) => setKeyword(e.target.value)}
            onKeyDown={onKeyDownInput}
          />
          <kbd className="command-search-kbd">Esc</kbd>
        </div>
        <div className="command-search-results">
          {items.length === 0 ? (
            <p className="command-search-empty">未找到匹配页面</p>
          ) : (
            items.map((item, index) => (
              <button
                key={item.href}
                type="button"
                className={index === activeIndex ? "command-search-item is-active" : "command-search-item"}
                onMouseEnter={() => setActiveIndex(index)}
                onClick={() => navigate(item.href)}
              >
                <span className="command-search-item-label">{item.label}</span>
                <span className="command-search-item-meta">{item.group}</span>
                <span className="command-search-item-path">{item.href}</span>
              </button>
            ))
          )}
        </div>
      </div>
    </div>,
    document.body,
  );
}
