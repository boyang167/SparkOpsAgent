"use client";

import { useEffect, useId, useRef, useState, useCallback } from "react";
import type { MemberCandidateVM } from "@sparkops/contracts/governance";
import { governance } from "@sparkops/web-sdk";

const MIN_QUERY_LEN = 2;
const SEARCH_DEBOUNCE_MS = 300;
const MAX_RESULTS = 10;

interface MemberInviteSearchInputProps {
  workspaceId?: string;
  value: string;
  onChange: (value: string) => void;
  onSelect?: (candidate: MemberCandidateVM) => void;
  searchCandidates?: (query: string, limit: number) => Promise<MemberCandidateVM[]>;
  placeholder?: string;
  disabled?: boolean;
  variant?: "default" | "admin" | "org";
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
  searchCandidates,
  placeholder = "搜索邮箱或姓名（须已注册）",
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
  const [searched, setSearched] = useState(false);

  const searchCandidatesRef = useRef(searchCandidates);
  searchCandidatesRef.current = searchCandidates;
  const workspaceIdRef = useRef(workspaceId);
  workspaceIdRef.current = workspaceId;

  const runSearch = useCallback((query: string, limit: number) => {
    const custom = searchCandidatesRef.current;
    if (custom) return custom(query, limit);
    const wsId = workspaceIdRef.current;
    if (wsId) return governance.searchMemberCandidates(wsId, query, limit);
    return Promise.resolve([] as MemberCandidateVM[]);
  }, []);

  useEffect(() => {
    const query = value.trim();
    const canSearch = Boolean(searchCandidatesRef.current || workspaceIdRef.current);
    if (!canSearch || query.length < MIN_QUERY_LEN) {
      setCandidates((prev) => (prev.length === 0 ? prev : []));
      setLoading((prev) => (prev ? false : prev));
      setOpen((prev) => (prev ? false : prev));
      setActiveIndex((prev) => (prev === -1 ? prev : -1));
      setSearched((prev) => (prev ? false : prev));
      return;
    }

    setLoading(true);
    let cancelled = false;
    const timer = window.setTimeout(() => {
      runSearch(query, MAX_RESULTS)
        .then((items) => {
          if (cancelled) return;
          setCandidates(items);
          setSearched(true);
          setOpen(true);
          setActiveIndex(items.length > 0 ? 0 : -1);
        })
        .catch(() => {
          if (cancelled) return;
          setCandidates([]);
          setSearched(true);
          setOpen(true);
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
  }, [value, runSearch]);

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

  const fieldClass =
    variant === "org"
      ? "tenant-invite-field"
      : "admin-members-invite-field";

  const inputClass = "admin-members-invite-input";

  const showResults = open && value.trim().length >= MIN_QUERY_LEN && !loading;
  const showHint = value.trim().length > 0 && value.trim().length < MIN_QUERY_LEN;
  const showEmpty = showResults && searched && candidates.length === 0;

  return (
    <div ref={rootRef} className={fieldClass}>
      <svg className="admin-members-invite-icon" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" aria-hidden>
        <circle cx="11" cy="11" r="7" />
        <path d="M20 20l-3.5-3.5" strokeLinecap="round" />
      </svg>

      <input
        id={inputId}
        className={inputClass}
        type="search"
        autoComplete="off"
        role="combobox"
        aria-expanded={showResults && candidates.length > 0}
        aria-controls={listId}
        aria-autocomplete="list"
        aria-activedescendant={activeIndex >= 0 ? `${listId}-${activeIndex}` : undefined}
        placeholder={placeholder}
        value={value}
        disabled={disabled}
        onChange={(event) => onChange(event.target.value)}
        onFocus={() => {
          if (value.trim().length >= MIN_QUERY_LEN) {
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

      {showResults && candidates.length > 0 ? (
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

      {showEmpty ? (
        <div className="member-invite-search-menu member-invite-search-empty" role="status">
          未找到可邀请的注册用户，或该用户已在当前范围内
        </div>
      ) : null}
    </div>
  );
}
