"use client";

import { useEffect, useRef, useState } from "react";
import { delivery } from "@sparkops/web-sdk";
import { formatDeploymentLogText } from "./deployment-log-format";

interface DeploymentLogStreamProps {
  runId: string;
  active: boolean;
  terminal?: boolean;
  onDone?: () => void;
}

const WAITING_LABEL = "等待日志…";

export function DeploymentLogStream({ runId, active: _active, terminal = false, onDone }: DeploymentLogStreamProps) {
  const [displayLog, setDisplayLog] = useState(WAITING_LABEL);
  const rawRef = useRef("");
  const logRef = useRef<HTMLPreElement>(null);
  const onDoneRef = useRef(onDone);
  onDoneRef.current = onDone;

  useEffect(() => {
    rawRef.current = "";
    setDisplayLog(WAITING_LABEL);

    const stop = delivery.streamDeployment(runId, {
      onDelta: (delta) => {
        rawRef.current += delta;
        const formatted = formatDeploymentLogText(rawRef.current);
        setDisplayLog(formatted || WAITING_LABEL);
      },
      onDone: () => {
        setDisplayLog((current) => {
          if (!rawRef.current.trim()) return "暂无日志";
          return formatDeploymentLogText(rawRef.current) || current;
        });
        onDoneRef.current?.();
      },
      onError: () => {
        setDisplayLog((current) => {
          if (rawRef.current.trim()) {
            return formatDeploymentLogText(rawRef.current);
          }
          return current === WAITING_LABEL ? "日志加载失败" : current;
        });
      },
    });

    return stop;
  }, [runId]);

  useEffect(() => {
    const el = logRef.current;
    if (!el) return;
    el.scrollTop = el.scrollHeight;
  }, [displayLog]);

  return (
    <pre
      ref={logRef}
      className={
        terminal
          ? "project-build-logs-terminal"
          : "text-xs bg-[var(--surface)] p-3 rounded-lg overflow-x-auto whitespace-pre-wrap max-h-96 overflow-y-auto font-mono"
      }
    >
      {displayLog}
    </pre>
  );
}
