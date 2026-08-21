"use client";

import { useEffect, useMemo, useRef, useState } from "react";
import { createPortal } from "react-dom";
import { useRouter } from "next/navigation";
import {
  COMMAND_KIND_LABELS,
  groupCommandResults,
  type CommandSearchItem,
} from "@/components/layout/command-search-index";
import { recordRecentVisit } from "@/components/layout/command-search-recent";
import { useShellContext } from "@/components/layout/shell-context";
import { useCommandSearch } from "@/components/layout/useCommandSearch";

interface CommandSearchModalProps {
  open: boolean;
  onClose: () => void;
}

function flattenSections(sections: ReturnType<typeof groupCommandResults>) {
  return sections.flatMap((section) => section.items);
}

export function CommandSearchModal({ open, onClose }: CommandSearchModalProps) {
  const router = useRouter();
  const inputRef = useRef<HTMLInputElement>(null);
  const { workspaceId, canManageAdmin, isSuperAdmin } = useShellContext();
  const [mounted, setMounted] = useState(false);
  const [activeIndex, setActiveIndex] = useState(0);

  const { keyword, setKeyword, results, loadingDynamic } = useCommandSearch({
    open,
    workspaceId,
    canManageAdmin,
    isSuperAdmin,
  });

  const grouped = useMemo(
    () => (keyword.trim() ? null : groupCommandResults(results)),
    [keyword, results],
  );

  const flatItems = useMemo(
    () => (grouped ? flattenSections(grouped) : results),
    [grouped, results],
  );

  useEffect(() => {
    setMounted(true);
  }, []);

  useEffect(() => {
    if (!open) return;
    setActiveIndex(0);
    window.setTimeout(() => inputRef.current?.focus(), 0);
  }, [open]);

  useEffect(() => {
    setActiveIndex(0);
  }, [keyword, results.length]);

  useEffect(() => {
    if (!open) return;
    function onKeyDown(e: KeyboardEvent) {
      if (e.key === "Escape") onClose();
    }
    document.addEventListener("keydown", onKeyDown);
    return () => document.removeEventListener("keydown", onKeyDown);
  }, [open, onClose]);

  if (!open || !mounted) return null;

  function navigate(item: CommandSearchItem) {
    recordRecentVisit({ href: item.href, label: item.label, group: item.group });
    onClose();
    router.push(item.href);
  }

  function onKeyDownInput(e: React.KeyboardEvent<HTMLInputElement>) {
    if (e.key === "ArrowDown") {
      e.preventDefault();
      setActiveIndex((prev) => Math.min(flatItems.length - 1, prev + 1));
    } else if (e.key === "ArrowUp") {
      e.preventDefault();
      setActiveIndex((prev) => Math.max(0, prev - 1));
    } else if (e.key === "Enter" && flatItems[activeIndex]) {
      e.preventDefault();
      navigate(flatItems[activeIndex]);
    }
  }

  let itemCounter = 0;

  function renderItem(item: CommandSearchItem) {
    const index = itemCounter;
    itemCounter += 1;
    return (
      <button
        key={`${item.kind}:${item.href}:${item.label}`}
        type="button"
        className={index === activeIndex ? "command-search-item is-active" : "command-search-item"}
        onMouseEnter={() => setActiveIndex(index)}
        onClick={() => navigate(item)}
      >
        <span className="command-search-item-main">
          <span className="command-search-item-label">{item.label}</span>
          <span className="command-search-item-meta">{item.group}</span>
        </span>
        <span className="command-search-item-trailing">
          <span className={`command-search-kind command-search-kind-${item.kind}`}>
            {COMMAND_KIND_LABELS[item.kind]}
          </span>
          <span className="command-search-item-path">{item.href}</span>
        </span>
      </button>
    );
  }

  return createPortal(
    <div className="command-search-root" role="presentation">
      <button type="button" className="command-search-backdrop" aria-label="关闭搜索" onClick={onClose} />
      <div className="command-search-panel" role="dialog" aria-modal="true" aria-label="命令面板">
        <div className="command-search-input-wrap">
          <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" aria-hidden>
            <circle cx="11" cy="11" r="7" />
            <path d="M20 20l-3-3" strokeLinecap="round" />
          </svg>
          <input
            ref={inputRef}
            className="command-search-input"
            placeholder="搜索菜单、项目、智能体、工作流、知识库…"
            value={keyword}
            onChange={(e) => setKeyword(e.target.value)}
            onKeyDown={onKeyDownInput}
          />
          <kbd className="command-search-kbd">Esc</kbd>
        </div>

        {loadingDynamic ? <p className="command-search-hint">正在索引当前工作区资源…</p> : null}

        <div className="command-search-results">
          {flatItems.length === 0 ? (
            <p className="command-search-empty">未找到匹配内容</p>
          ) : grouped ? (
            grouped.map((section) => (
              <section key={section.kind} className="command-search-section">
                <h3 className="command-search-section-title">{section.label}</h3>
                <div className="command-search-section-items">{section.items.map((item) => renderItem(item))}</div>
              </section>
            ))
          ) : (
            flatItems.map((item) => renderItem(item))
          )}
        </div>
      </div>
    </div>,
    document.body,
  );
}
