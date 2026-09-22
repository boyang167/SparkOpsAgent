"use client";

import { useEffect, useRef } from "react";
import { delivery } from "@sparkops/web-sdk";

interface DeploymentLogStreamProps {
  runId: string;
  active: boolean;
  terminal?: boolean;
  onDone?: () => void;
}

export function DeploymentLogStream({ runId, active, terminal = false, onDone }: DeploymentLogStreamProps) {
  const logRef = useRef<HTMLPreElement>(null);
  const logTextRef = useRef("");

  useEffect(() => {
    logTextRef.current = "";
    if (logRef.current) {
      logRef.current.textContent = "等待日志…";
    }
  }, [runId]);

  useEffect(() => {
    if (!active) {
      return;
    }

    const stop = delivery.streamDeployment(runId, {
      onDelta: (delta) => {
        logTextRef.current += delta;
        if (logRef.current) {
          logRef.current.textContent = logTextRef.current;
          logRef.current.scrollTop = logRef.current.scrollHeight;
        }
      },
      onDone: () => onDone?.(),
      onError: () => onDone?.(),
    });

    return stop;
  }, [runId, active, onDone]);

  return (
    <pre
      ref={logRef}
      className={
        terminal
          ? "project-build-logs-terminal"
          : "text-xs bg-[var(--surface)] p-3 rounded-lg overflow-x-auto whitespace-pre-wrap max-h-96 overflow-y-auto font-mono"
      }
    >
      {active ? "等待日志…" : "暂无实时日志"}
    </pre>
  );
}
