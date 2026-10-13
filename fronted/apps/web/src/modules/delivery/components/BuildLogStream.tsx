"use client";

import { useEffect, useRef, useState } from "react";
import { delivery } from "@sparkops/web-sdk";
import {
  countLogLines,
  countNewlines,
  createConsoleLogSanitizer,
  sanitizeConsoleLog,
  tailLines,
} from "./console-log";

export interface BuildLogStats {
  lines: number;
  getText: () => string;
}

interface BuildLogStreamProps {
  buildId: string;
  initialLog?: string;
  active: boolean;
  terminal?: boolean;
  className?: string;
  emptyText?: string;
  maxLines?: number;
  onDone?: () => void;
  onStatsChange?: (stats: BuildLogStats) => void;
}

const STICK_THRESHOLD_PX = 48;
const DEFAULT_MAX_LINES = 5000;
const STATS_THROTTLE_MS = 300;

function downloadText(text: string, filename: string) {
  const url = URL.createObjectURL(new Blob([text], { type: "text/plain;charset=utf-8" }));
  const anchor = document.createElement("a");
  anchor.href = url;
  anchor.download = filename;
  anchor.click();
  window.setTimeout(() => URL.revokeObjectURL(url), 1000);
}

export function BuildLogStream({
  buildId,
  initialLog,
  active,
  terminal = false,
  className,
  emptyText = "等待日志…",
  maxLines = DEFAULT_MAX_LINES,
  onDone,
  onStatsChange,
}: BuildLogStreamProps) {
  const logRef = useRef<HTMLPreElement>(null);
  const fullRef = useRef("");
  const shownLinesRef = useRef(0);
  const receivedDeltaRef = useRef(false);
  const statsTimerRef = useRef<number | null>(null);
  const onStatsRef = useRef(onStatsChange);
  const onDoneRef = useRef(onDone);
  onStatsRef.current = onStatsChange;
  onDoneRef.current = onDone;
  const [hiddenLines, setHiddenLines] = useState(0);
  const [empty, setEmpty] = useState(true);

  function emitStats(immediate = false) {
    const emit = () => {
      statsTimerRef.current = null;
      onStatsRef.current?.({ lines: countLogLines(fullRef.current), getText: () => fullRef.current });
    };
    if (immediate) {
      if (statsTimerRef.current !== null) window.clearTimeout(statsTimerRef.current);
      emit();
      return;
    }
    if (statsTimerRef.current === null) statsTimerRef.current = window.setTimeout(emit, STATS_THROTTLE_MS);
  }

  function renderTail() {
    const el = logRef.current;
    if (!el) return;
    const tail = tailLines(fullRef.current, maxLines);
    el.textContent = tail.text;
    shownLinesRef.current = countLogLines(tail.text);
    setHiddenLines(tail.hidden);
    setEmpty(fullRef.current.length === 0);
  }

  function appendClean(chunk: string) {
    if (!chunk) return;
    fullRef.current += chunk;
    const el = logRef.current;
    if (!el) return;
    const nearBottom = el.scrollHeight - el.scrollTop - el.clientHeight < STICK_THRESHOLD_PX;
    shownLinesRef.current += countNewlines(chunk);
    if (shownLinesRef.current > maxLines * 1.2) {
      renderTail();
    } else {
      el.appendChild(document.createTextNode(chunk));
      setEmpty(false);
    }
    if (nearBottom) el.scrollTop = el.scrollHeight;
    emitStats();
  }

  useEffect(() => {
    fullRef.current = sanitizeConsoleLog(initialLog ?? "");
    receivedDeltaRef.current = false;
    renderTail();
    emitStats(true);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [buildId, initialLog, maxLines]);

  useEffect(() => {
    if (!active) return;
    const sanitizer = createConsoleLogSanitizer();
    let firstDelta = true;

    const stop = delivery.streamBuild(buildId, {
      onDelta: (delta) => {
        const clean = sanitizer.push(delta);
        if (firstDelta && clean) {
          firstDelta = false;
          // The stream opens with a snapshot of everything buffered so far, which overlaps initialLog.
          const existing = fullRef.current;
          const probe = Math.min(200, existing.length, clean.length);
          if (existing && existing.slice(0, probe) === clean.slice(0, probe)) {
            if (clean.length > existing.length) {
              fullRef.current = clean;
              renderTail();
              const el = logRef.current;
              if (el) el.scrollTop = el.scrollHeight;
              emitStats();
            }
            return;
          }
        }
        appendClean(clean);
      },
      onDone: () => {
        appendClean(sanitizer.flush());
        emitStats(true);
        onDoneRef.current?.();
      },
    });

    return () => {
      stop();
      appendClean(sanitizer.flush());
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [buildId, active]);

  useEffect(
    () => () => {
      if (statsTimerRef.current !== null) window.clearTimeout(statsTimerRef.current);
    },
    [],
  );

  const resolvedClass =
    className ??
    (terminal
      ? "project-build-logs-terminal"
      : "text-xs bg-[var(--surface)] p-3 rounded-lg overflow-x-auto whitespace-pre-wrap max-h-96 overflow-y-auto font-mono");

  return (
    <>
      {hiddenLines > 0 ? (
        <div className="build-log-truncated">
          <span>
            日志较长，仅显示最后 {maxLines.toLocaleString()} 行（已省略前 {hiddenLines.toLocaleString()} 行）
          </span>
          <button
            type="button"
            className="build-log-truncated-btn"
            onClick={() => downloadText(fullRef.current, `build-${buildId.slice(0, 8)}.log`)}
          >
            下载完整日志
          </button>
        </div>
      ) : null}
      <pre ref={logRef} className={`build-log-pre ${resolvedClass}`} data-empty={empty ? emptyText : undefined} />
    </>
  );
}
