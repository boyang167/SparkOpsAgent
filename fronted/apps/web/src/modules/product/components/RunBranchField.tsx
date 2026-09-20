"use client";

import { useCallback, useEffect, useId, useLayoutEffect, useMemo, useRef, useState } from "react";
import { createPortal } from "react-dom";
import { BffError } from "@sparkops/contracts";
import { workspace } from "@sparkops/web-sdk";

type RefKind = "branch" | "tag" | "pr";

interface RefOption {
  name: string;
  title?: string;
}

interface RunBranchFieldProps {
  workspaceId: string;
  gitUrl: string;
  secretId: string;
  value: string;
  refType: RefKind;
  label: string;
  onChange: (value: string, refType: RefKind) => void;
}

function formatRefLabel(item: RefOption, kind: RefKind): string {
  if (kind === "pr") {
    const title = item.title?.trim();
    return title ? `#${item.name} ${title}` : `#${item.name}`;
  }
  return item.name;
}

function formatRefValue(value: string, kind: RefKind, options: RefOption[]): string {
  if (!value) return "";
  if (kind !== "pr") return value;
  const match = options.find((item) => item.name === value);
  return match ? formatRefLabel(match, kind) : `#${value}`;
}

function readLoadError(err: unknown, kind: RefKind): string {
  const message = err instanceof BffError || err instanceof Error ? err.message : "加载失败";
  if (kind === "pr" && message.includes("unsupported ref type")) {
    return "PR 列表暂不可用，请重启完整后端服务";
  }
  return message;
}

function mapRefItems(items: { name: string; title?: string }[]): RefOption[] {
  return items.filter((item) => item.name).map((item) => ({ name: item.name, title: item.title }));
}

export function RunBranchField({
  workspaceId,
  gitUrl,
  secretId,
  value,
  refType,
  label,
  onChange,
}: RunBranchFieldProps) {
  const [mode, setMode] = useState<"select" | "manual">("select");
  const [tab, setTab] = useState<RefKind>(refType);
  const [branches, setBranches] = useState<RefOption[]>([]);
  const [tags, setTags] = useState<RefOption[]>([]);
  const [pullRequests, setPullRequests] = useState<RefOption[]>([]);
  const [loadingCore, setLoadingCore] = useState(false);
  const [loadingPr, setLoadingPr] = useState(false);
  const [prLoaded, setPrLoaded] = useState(false);
  const [branchError, setBranchError] = useState<string | null>(null);
  const [tagError, setTagError] = useState<string | null>(null);
  const [prError, setPrError] = useState<string | null>(null);
  const [open, setOpen] = useState(false);
  const [pendingOpen, setPendingOpen] = useState(false);
  const [query, setQuery] = useState(value);
  const [menuStyle, setMenuStyle] = useState<{ top: number; left: number; width: number; maxHeight: number } | null>(
    null,
  );
  const rootRef = useRef<HTMLDivElement>(null);
  const inputRef = useRef<HTMLInputElement>(null);
  const manualEntryRef = useRef(false);
  const prLoadingRef = useRef(false);
  const listId = useId();
  const canLoad = Boolean(workspaceId && gitUrl.trim() && secretId.trim());

  useEffect(() => {
    setQuery(value);
    setTab(refType);
  }, [value, refType]);

  const loadPullRequests = useCallback(async () => {
    if (!canLoad || loadingPr) return;
    setLoadingPr(true);
    setPrError(null);
    const payload = { git_url: gitUrl.trim(), secret_id: secretId.trim() };
    try {
      const prItems = await workspace.listGitRefs(workspaceId, { ...payload, ref_type: "pr" });
      setPullRequests(mapRefItems(prItems));
      setPrLoaded(true);
    } catch (err: unknown) {
      setPullRequests([]);
      setPrError(readLoadError(err, "pr"));
    } finally {
      setLoadingPr(false);
    }
  }, [canLoad, loadingPr, workspaceId, gitUrl, secretId]);

  useEffect(() => {
    if (!canLoad) {
      setBranches([]);
      setTags([]);
      setPullRequests([]);
      setPrLoaded(false);
      setBranchError("缺少仓库地址或密钥，无法拉取分支");
      setTagError(null);
      setPrError(null);
      return;
    }
    let cancelled = false;
    setLoadingCore(true);
    setBranchError(null);
    setTagError(null);
    setPrError(null);
    setPullRequests([]);
    setPrLoaded(false);
    const payload = { git_url: gitUrl.trim(), secret_id: secretId.trim() };
    void Promise.allSettled([
      workspace.listGitRefs(workspaceId, { ...payload, ref_type: "branch" }),
      workspace.listGitRefs(workspaceId, { ...payload, ref_type: "tag" }),
    ])
      .then((results) => {
        if (cancelled) return;
        const [branchResult, tagResult] = results;
        const tagNames =
          tagResult.status === "fulfilled"
            ? new Set(tagResult.value.map((item) => item.name).filter(Boolean))
            : new Set<string>();

        if (branchResult.status === "fulfilled") {
          setBranches(
            branchResult.value
              .filter((item) => item.name && !tagNames.has(item.name))
              .map((item) => ({ name: item.name, title: item.title })),
          );
          setBranchError(null);
        } else {
          setBranches([]);
          setBranchError(readLoadError(branchResult.reason, "branch"));
        }

        if (tagResult.status === "fulfilled") {
          setTags(mapRefItems(tagResult.value));
          setTagError(null);
        } else {
          setTags([]);
          setTagError(readLoadError(tagResult.reason, "tag"));
        }
      })
      .finally(() => {
        if (!cancelled) setLoadingCore(false);
      });
    return () => {
      cancelled = true;
    };
  }, [canLoad, workspaceId, gitUrl, secretId]);

  useEffect(() => {
    if (!canLoad || tab !== "pr" && refType !== "pr") return;
    void loadPullRequests();
  }, [canLoad, tab, refType, loadPullRequests]);

  const options = tab === "tag" ? tags : tab === "pr" ? pullRequests : branches;
  const items = useMemo(() => {
    const keyword = query.trim().toLowerCase();
    if (!keyword) return options;
    return options.filter((item) => {
      const labelText = formatRefLabel(item, tab).toLowerCase();
      return labelText.includes(keyword) || item.name.toLowerCase().includes(keyword);
    });
  }, [options, query, tab]);

  const displayValue = useMemo(() => {
    if (open) return query;
    if (refType === "pr") return formatRefValue(value, "pr", pullRequests);
    return value;
  }, [open, query, refType, value, pullRequests]);

  function placeMenu() {
    const rect = rootRef.current?.getBoundingClientRect();
    if (!rect) return;
    const spaceBelow = window.innerHeight - rect.bottom - 12;
    const spaceAbove = rect.top - 12;
    const openUp = spaceBelow < 200 && spaceAbove > spaceBelow;
    const maxHeight = Math.max(160, Math.min(280, openUp ? spaceAbove : spaceBelow));
    setMenuStyle({
      top: openUp ? Math.max(8, rect.top - maxHeight - 4) : rect.bottom + 4,
      left: rect.left,
      width: Math.max(rect.width, 260),
      maxHeight,
    });
  }

  useLayoutEffect(() => {
    if (!open) return;
    placeMenu();
  }, [open]);

  useLayoutEffect(() => {
    if (mode !== "select" || !pendingOpen) return;
    setQuery(value);
    placeMenu();
    setOpen(true);
    setPendingOpen(false);
  }, [mode, pendingOpen, value]);

  function enterManualMode() {
    setOpen(false);
    setMode("manual");
    manualEntryRef.current = true;
    requestAnimationFrame(() => {
      inputRef.current?.focus();
      manualEntryRef.current = false;
    });
  }

  function enterSelectModeAndOpen() {
    setMode("select");
    setPendingOpen(true);
  }

  function openDropdown() {
    setQuery("");
    placeMenu();
    setOpen(true);
  }

  useEffect(() => {
    if (!open) return;
    function onPointer(event: MouseEvent) {
      const target = event.target;
      if (target instanceof Node && rootRef.current?.contains(target)) return;
      if (target instanceof Element && target.closest(`[data-menu="${listId}"]`)) return;
      setOpen(false);
      setQuery(value);
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
  }, [open, listId, value]);

  if (mode === "manual") {
    return (
      <div className="wf-run-branch-field">
        <input
          ref={inputRef}
          className="input wf-run-branch"
          value={value}
          aria-label={label}
          placeholder="输入分支、Tag 或 PR 编号"
          onChange={(event) => onChange(event.target.value, tab)}
          onFocus={() => {
            if (manualEntryRef.current) return;
            enterSelectModeAndOpen();
          }}
        />
      </div>
    );
  }

  const tabError = tab === "tag" ? tagError : tab === "pr" ? prError : branchError;
  const tabLoading = tab === "pr" ? loadingPr : loadingCore;
  const emptyLabel =
    tab === "tag" ? "没有匹配的 Tag" : tab === "pr" ? "没有匹配的 PR" : "没有匹配的分支";

  return (
    <div className="wf-run-branch-field">
      <div className="pwf-combo wf-run-combo" ref={rootRef}>
        <input
          className="input wf-run-branch"
          value={displayValue}
          aria-label={label}
          aria-expanded={open}
          aria-controls={listId}
          placeholder={loadingCore ? "加载分支…" : "请选择分支或 PR"}
          onClick={() => {
            if (!open) openDropdown();
          }}
          onFocus={() => {
            if (open) return;
            openDropdown();
          }}
          onChange={(event) => {
            setQuery(event.target.value);
            if (!open) {
              placeMenu();
              setOpen(true);
            }
          }}
          onKeyDown={(event) => {
            if (event.key === "Enter" && items[0]) {
              event.preventDefault();
              onChange(items[0].name, tab);
              setQuery(items[0].name);
              setOpen(false);
            }
            if (event.key === "Escape") {
              setOpen(false);
              setQuery(value);
            }
          }}
        />
        {open && menuStyle
          ? createPortal(
              <div className="wf-run-ref-menu" data-menu={listId} style={menuStyle}>
                <div className="wf-run-ref-tabs">
                  <button
                    type="button"
                    className={`wf-run-ref-tab${tab === "pr" ? " is-active" : ""}`}
                    onMouseDown={(event) => event.preventDefault()}
                    onClick={() => {
                      setTab("pr");
                      setQuery("");
                      void loadPullRequests();
                    }}
                  >
                    PR
                  </button>
                  <button
                    type="button"
                    className={`wf-run-ref-tab${tab === "branch" ? " is-active" : ""}`}
                    onMouseDown={(event) => event.preventDefault()}
                    onClick={() => {
                      setTab("branch");
                      setQuery("");
                    }}
                  >
                    Branches
                  </button>
                  <button
                    type="button"
                    className={`wf-run-ref-tab${tab === "tag" ? " is-active" : ""}`}
                    onMouseDown={(event) => event.preventDefault()}
                    onClick={() => {
                      setTab("tag");
                      setQuery("");
                    }}
                  >
                    Tags
                  </button>
                </div>
                <ul id={listId} className="wf-run-ref-list" role="listbox">
                  {tabLoading && items.length === 0 ? (
                    <li className="pwf-combo-empty">加载中…</li>
                  ) : tabError && items.length === 0 ? (
                    <li className="pwf-combo-empty">{tabError}</li>
                  ) : items.length === 0 ? (
                    <li className="pwf-combo-empty">{emptyLabel}</li>
                  ) : (
                    items.map((item) => (
                      <li key={`${tab}-${item.name}`}>
                        <button
                          type="button"
                          role="option"
                          aria-selected={item.name === value && refType === tab}
                          className={item.name === value && refType === tab ? "is-active" : undefined}
                          onMouseDown={(event) => event.preventDefault()}
                          onClick={() => {
                            onChange(item.name, tab);
                            setQuery(item.name);
                            setOpen(false);
                          }}
                        >
                          {formatRefLabel(item, tab)}
                        </button>
                      </li>
                    ))
                  )}
                </ul>
              </div>,
              document.body,
            )
          : null}
      </div>
      <button type="button" className="wf-run-branch-mode" onClick={enterManualMode}>
        手动输入
      </button>
    </div>
  );
}
