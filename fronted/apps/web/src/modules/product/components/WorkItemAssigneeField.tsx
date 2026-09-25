"use client";

import { useEffect, useId, useMemo, useRef, useState } from "react";
import { createPortal } from "react-dom";
import { governance } from "@sparkops/web-sdk";
import type { WorkspaceMemberVM } from "@sparkops/contracts/governance";

interface WorkItemAssigneeFieldProps {
  workspaceId: string;
  assigneeId?: string;
  assigneeName?: string;
  disabled?: boolean;
  onSelect: (memberId: string | null, label: string) => void | Promise<void>;
}

function memberLabel(member: WorkspaceMemberVM): string {
  return member.display_name?.trim() || member.email || member.member_id;
}

function matchesQuery(member: WorkspaceMemberVM, query: string): boolean {
  const q = query.trim().toLowerCase();
  if (!q) return true;
  return (
    member.display_name.toLowerCase().includes(q) ||
    member.email.toLowerCase().includes(q)
  );
}

export function WorkItemAssigneeField({
  workspaceId,
  assigneeId,
  assigneeName,
  disabled = false,
  onSelect,
}: WorkItemAssigneeFieldProps) {
  const listId = useId();
  const rootRef = useRef<HTMLDivElement>(null);
  const searchRef = useRef<HTMLInputElement>(null);
  const [open, setOpen] = useState(false);
  const [query, setQuery] = useState("");
  const [members, setMembers] = useState<WorkspaceMemberVM[]>([]);
  const [loading, setLoading] = useState(false);
  const [menuStyle, setMenuStyle] = useState<{ top: number; left: number; width: number; maxHeight: number } | null>(
    null,
  );

  const displayLabel = assigneeName?.trim() || (assigneeId ? "已分配成员" : "未分配");

  const filteredMembers = useMemo(
    () => members.filter((member) => matchesQuery(member, query)),
    [members, query],
  );

  const showUnassigned = !query.trim() || "未分配".includes(query.trim().toLowerCase());

  useEffect(() => {
    if (!workspaceId) return;
    let cancelled = false;
    setLoading(true);
    void governance
      .listMembers(workspaceId)
      .then((items) => {
        if (!cancelled) setMembers(items);
      })
      .catch(() => {
        if (!cancelled) setMembers([]);
      })
      .finally(() => {
        if (!cancelled) setLoading(false);
      });
    return () => {
      cancelled = true;
    };
  }, [workspaceId]);

  function placeMenu() {
    const rect = rootRef.current?.getBoundingClientRect();
    if (!rect) return;
    const spaceBelow = window.innerHeight - rect.bottom - 12;
    const spaceAbove = rect.top - 12;
    const openUp = spaceBelow < 220 && spaceAbove > spaceBelow;
    const maxHeight = Math.max(160, Math.min(280, openUp ? spaceAbove : spaceBelow));
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
      setQuery("");
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

  useEffect(() => {
    if (!open) return;
    placeMenu();
    const timer = window.setTimeout(() => searchRef.current?.focus(), 0);
    return () => window.clearTimeout(timer);
  }, [open]);

  async function pick(memberId: string | null, label: string) {
    setOpen(false);
    setQuery("");
    await onSelect(memberId, label);
  }

  return (
    <div className="pwf-combo work-item-assignee-field" ref={rootRef}>
      <button
        type="button"
        className="input pwf-combo-trigger"
        aria-label="负责人"
        aria-expanded={open}
        aria-controls={listId}
        disabled={disabled || loading}
        onClick={() => {
          if (disabled) return;
          if (open) {
            setOpen(false);
            setQuery("");
            return;
          }
          placeMenu();
          setOpen(true);
        }}
      >
        <span className={assigneeId ? undefined : "text-muted-foreground"}>{displayLabel}</span>
        <span aria-hidden>{open ? "▴" : "▾"}</span>
      </button>
      {open && menuStyle
        ? createPortal(
            <div
              data-menu={listId}
              className="pwf-combo-menu work-item-assignee-menu"
              style={menuStyle}
            >
              <div className="pwf-combo-search">
                <input
                  ref={searchRef}
                  className="input text-sm"
                  type="search"
                  placeholder="搜索姓名或邮箱"
                  value={query}
                  onChange={(e) => setQuery(e.target.value)}
                  onKeyDown={(e) => {
                    if (e.key === "Escape") {
                      setOpen(false);
                      setQuery("");
                    }
                  }}
                />
              </div>
              <ul id={listId} role="listbox" className="work-item-assignee-list">
                {showUnassigned ? (
                  <li>
                    <button
                      type="button"
                      role="option"
                      aria-selected={!assigneeId}
                      className={!assigneeId ? "is-active" : undefined}
                      onClick={() => void pick(null, "未分配")}
                    >
                      未分配
                    </button>
                  </li>
                ) : null}
                {filteredMembers.length === 0 ? (
                  <li className="pwf-combo-empty">{loading ? "加载中…" : "无匹配成员"}</li>
                ) : (
                  filteredMembers.map((member) => {
                    const label = memberLabel(member);
                    const active = member.member_id === assigneeId;
                    return (
                      <li key={member.member_id}>
                        <button
                          type="button"
                          role="option"
                          aria-selected={active}
                          className={active ? "is-active" : undefined}
                          onClick={() => void pick(member.member_id, label)}
                        >
                          <span className="work-item-assignee-option">
                            <span className="work-item-assignee-name">{label}</span>
                            {member.email && member.display_name ? (
                              <span className="work-item-assignee-email">{member.email}</span>
                            ) : null}
                          </span>
                        </button>
                      </li>
                    );
                  })
                )}
              </ul>
            </div>,
            document.body,
          )
        : null}
    </div>
  );
}
