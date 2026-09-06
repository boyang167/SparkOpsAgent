"use client";

import { useEffect, useRef } from "react";
import { delivery } from "@sparkops/web-sdk";

interface BuildLogStreamProps {
  buildId: string;
  initialLog?: string;
  active: boolean;
  terminal?: boolean;
  onDone?: () => void;
}

export function BuildLogStream({ buildId, initialLog, active, terminal = false, onDone }: BuildLogStreamProps) {
  const logRef = useRef<HTMLPreElement>(null);
  const logTextRef = useRef(initialLog ?? "");

  useEffect(() => {
    if (initialLog) {
      logTextRef.current = initialLog;
      if (logRef.current) {
        logRef.current.textContent = initialLog;
      }
    }
  }, [initialLog]);

  useEffect(() => {
    if (!active) {
      return;
    }

    const stop = delivery.streamBuild(buildId, {
      onDelta: (delta) => {
        logTextRef.current += delta;
        if (logRef.current) {
          logRef.current.textContent = logTextRef.current;
          logRef.current.scrollTop = logRef.current.scrollHeight;
        }
      },
      onDone: () => onDone?.(),
    });

    return stop;
  }, [buildId, active, onDone]);

  return (
    <pre
      ref={logRef}
      className={
        terminal
          ? "project-build-logs-terminal"
          : "text-xs bg-[var(--surface)] p-3 rounded-lg overflow-x-auto whitespace-pre-wrap max-h-96 overflow-y-auto font-mono"
      }
    >
      {logTextRef.current || "等待日志…"}
    </pre>
  );
}
