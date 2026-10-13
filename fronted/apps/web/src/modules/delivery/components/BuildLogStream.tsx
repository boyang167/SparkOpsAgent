"use client";

import { useEffect, useRef } from "react";
import { delivery } from "@sparkops/web-sdk";
import { sanitizeConsoleLog } from "./console-log";

interface BuildLogStreamProps {
  buildId: string;
  initialLog?: string;
  active: boolean;
  terminal?: boolean;
  className?: string;
  emptyText?: string;
  onDone?: () => void;
  onTextChange?: (text: string) => void;
}

const STICK_THRESHOLD_PX = 48;

export function BuildLogStream({
  buildId,
  initialLog,
  active,
  terminal = false,
  className,
  emptyText = "等待日志…",
  onDone,
  onTextChange,
}: BuildLogStreamProps) {
  const logRef = useRef<HTMLPreElement>(null);
  const rawRef = useRef(initialLog ?? "");
  const onTextChangeRef = useRef(onTextChange);
  onTextChangeRef.current = onTextChange;

  function render(stickToBottom: boolean) {
    const text = sanitizeConsoleLog(rawRef.current);
    const el = logRef.current;
    if (el) {
      const nearBottom = el.scrollHeight - el.scrollTop - el.clientHeight < STICK_THRESHOLD_PX;
      el.textContent = text || emptyText;
      if (stickToBottom && nearBottom) el.scrollTop = el.scrollHeight;
    }
    onTextChangeRef.current?.(text);
  }

  useEffect(() => {
    if (initialLog) {
      rawRef.current = initialLog;
      render(false);
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [initialLog]);

  useEffect(() => {
    if (!active) {
      return;
    }

    const stop = delivery.streamBuild(buildId, {
      onDelta: (delta) => {
        rawRef.current += delta;
        render(true);
      },
      onDone: () => onDone?.(),
    });

    return stop;
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [buildId, active, onDone]);

  const resolvedClass =
    className ??
    (terminal
      ? "project-build-logs-terminal"
      : "text-xs bg-[var(--surface)] p-3 rounded-lg overflow-x-auto whitespace-pre-wrap max-h-96 overflow-y-auto font-mono");

  return (
    <pre ref={logRef} className={resolvedClass}>
      {sanitizeConsoleLog(rawRef.current) || emptyText}
    </pre>
  );
}
