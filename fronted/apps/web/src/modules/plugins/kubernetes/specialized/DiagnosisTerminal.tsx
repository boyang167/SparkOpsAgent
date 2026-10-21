"use client";

import { useEffect, useRef } from "react";
import { createSSEParser, formatSSEPayload } from "../sse";
import { execUrl, type DiagnosisTarget } from "../diagnosis-view";

export function DiagnosisTerminal({ target, onClose }: { target: DiagnosisTarget; onClose: () => void }) {
  const containerRef = useRef<HTMLPreElement>(null);
  const linesRef = useRef<string[]>([]);

  useEffect(() => {
    if (!target.clusterId || !target.namespace || !target.pod || !target.container) return;
    const command = [
      "/bin/sh",
      "-c",
      "java -jar /tmp/arthas-bin/arthas-client.jar 127.0.0.1 3658; export LINES=40; export COLUMNS=120",
    ];
    const source = new EventSource(execUrl(target.clusterId, target, command));
    const append = (line: string) => {
      linesRef.current = [...linesRef.current.slice(-1000), line];
      if (containerRef.current) {
        containerRef.current.textContent = linesRef.current.join("\n");
      }
    };
    const parser = createSSEParser((event) => {
      if (event.event === "output" || event.event === "error" || event.event === "message") {
        append(formatSSEPayload(event.data));
      }
      if (event.event === "exit") {
        source.close();
      }
    });
    source.onmessage = (event) => parser.push(event.data);
    source.addEventListener("output", (event) => parser.push((event as MessageEvent).data));
    source.addEventListener("error", (event) => parser.push((event as MessageEvent).data));
    source.addEventListener("exit", () => source.close());
    return () => source.close();
  }, [target.clusterId, target.container, target.namespace, target.pod]);

  return (
    <div className="stack-md">
      <pre ref={containerRef} className="input input-mono min-h-80 whitespace-pre-wrap text-xs overflow-auto" />
      <div className="flex justify-end">
        <button type="button" className="btn btn-ghost" onClick={onClose}>
          关闭
        </button>
      </div>
    </div>
  );
}
