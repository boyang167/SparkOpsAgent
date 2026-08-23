"use client";

import { useEffect, useId, useRef, useState } from "react";
import type { MemberCandidateVM } from "@sparkops/contracts/governance";
import { governance } from "@sparkops/web-sdk";

const MIN_QUERY_LEN = 2;
const SEARCH_DEBOUNCE_MS = 300;
const MAX_RESULTS = 10;

interface MemberInviteSearchInputProps {
  workspaceId: string;
  value: string;
  onChange: (value: string) => void;
  onSelect?: (candidate: MemberCandidateVM) => void;
  placeholder?: string;
  disabled?: boolean;
  variant?: "default" | "admin";
  inputId?: string;
}

function userInitial(name: string, email: string) {
  const source = name.trim() || email.trim();
  return source.charAt(0).toUpperCase();
}

export function MemberInviteSearchInput({
  workspaceId,
  value,
  onChange,
  onSelect,
  placeholder = "输入邮箱或姓名搜索成员",
  disabled = false,
  variant = "default",
  inputId,
}: MemberInviteSearchInputProps) {
  const listId = useId();
  const rootRef = useRef<HTMLDivElement>(null);
  const [open, setOpen] = useState(false);
  const [loading, setLoading] = useState(false);
  const [candidates, setCandidates] = useState<MemberCandidateVM[]>([]);
  const [activeIndex, setActiveIndex] = useState(-1);

  useEffect(() => {
    const query = value.trim();
    if (query.length < MIN_QUERY_LEN) {
      setCandidates([]);
      setLoading(false);
      setOpen(false);
      setActiveIndex(-1);
      return;
    }

    setLoading(true);
    let cancelled = false;
    const timer = window.setTimeout(() => {
      governance
        .searchMemberCandidates(workspaceId, query, MAX_RESULTS)
        .then((items) => {
          if (cancelled) return;
          setCandidates(items);
          setOpen(items.length > 0);
          setActiveIndex(items.length > 0 ? 0 : -1);
        })
        .catch(() => {
          if (cancelled) return;
          setCandidates([]);
          setOpen(false);
          setActiveIndex(-1);
        })
        .finally(() => {
          if (!cancelled) {
            setLoading(false);
          }
        });
    }, SEARCH_DEBOUNCE_MS);

    return () => {
      cancelled = true;
      window.clearTimeout(timer);
    };
  }, [value, workspaceId]);

  useEffect(() => {
    function onPointerDown(event: MouseEvent) {
      if (!rootRef.current?.contains(event.target as Node)) {
        setOpen(false);
      }
    }
    document.addEventListener("mousedown", onPointerDown);
    return () => document.removeEventListener("mousedown", onPointerDown);
  }, []);

  function pickCandidate(candidate: MemberCandidateVM) {
    onChange(candidate.email);
    onSelect?.(candidate);
    setOpen(false);
    setActiveIndex(-1);
  }

  function onKeyDown(event: React.KeyboardEvent<HTMLInputElement>) {
    if (!open || candidates.length === 0) {
      if (event.key === "Escape") {
        setOpen(false);
      }
      return;
    }

    if (event.key === "ArrowDown") {
      event.preventDefault();
      setActiveIndex((index) => (index + 1) % candidates.length);
      return;
    }
    if (event.key === "ArrowUp") {
      event.preventDefault();
      setActiveIndex((index) => (index <= 0 ? candidates.length - 1 : index - 1));
      return;
    }
    if (event.key === "Enter" && activeIndex >= 0) {
      event.preventDefault();
      pickCandidate(candidates[activeIndex]!);
      return;
    }
    if (event.key === "Escape") {
      setOpen(false);
    }
  }

  const inputClass =
    variant === "admin" ? "admin-members-invite-input" : "input flex-1";

  const showPanel = open && candidates.length > 0;
  const showHint = value.trim().length > 0 && value.trim().length < MIN_QUERY_LEN;

  return (
    <div
      ref={rootRef}
      className={variant === "admin" ? "admin-members-invite-field" : "member-invite-search relative flex-1"}
    >
      {variant === "admin" ? (
        <svg className="admin-members-invite-icon" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" aria-hidden>
          <circle cx="11" cy="11" r="7" />
          <path d="M20 20l-3.5-3.5" strokeLinecap="round" />
        </svg>
      ) : null}

      <input
        id={inputId}
        className={inputClass}
        type="search"
        autoComplete="off"
        role="combobox"
        aria-expanded={showPanel}
        aria-controls={listId}
        aria-autocomplete="list"
        aria-activedescendant={activeIndex >= 0 ? `${listId}-${activeIndex}` : undefined}
        placeholder={placeholder}
        value={value}
        disabled={disabled}
        onChange={(event) => onChange(event.target.value)}
        onFocus={() => {
          if (candidates.length > 0) {
            setOpen(true);
          }
        }}
        onKeyDown={onKeyDown}
      />

      {showHint ? (
        <p className="member-invite-search-hint">至少输入 {MIN_QUERY_LEN} 个字符开始搜索</p>
      ) : null}

      {loading && value.trim().length >= MIN_QUERY_LEN ? (
        <span className="member-invite-search-loading" aria-hidden>
          搜索中…
        </span>
      ) : null}

      {showPanel ? (
        <ul id={listId} className="member-invite-search-menu" role="listbox">
          {candidates.map((candidate, index) => (
            <li key={candidate.member_id} role="presentation">
              <button
                id={`${listId}-${index}`}
                type="button"
                role="option"
                aria-selected={index === activeIndex}
                className={`member-invite-search-item ${index === activeIndex ? "member-invite-search-item-active" : ""}`}
                onMouseEnter={() => setActiveIndex(index)}
                onMouseDown={(event) => event.preventDefault()}
                onClick={() => pickCandidate(candidate)}
              >
                <span className="member-invite-search-avatar" aria-hidden>
                  {userInitial(candidate.display_name, candidate.email)}
                </span>
                <span className="member-invite-search-meta">
                  <span className="member-invite-search-name">{candidate.display_name}</span>
                  <span className="member-invite-search-email">{candidate.email}</span>
                </span>
              </button>
            </li>
          ))}
        </ul>
      ) : null}
    </div>
  );
}
