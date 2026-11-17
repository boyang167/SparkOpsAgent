"use client";

import { createElement, useEffect, useRef, useState } from "react";
import type { FileContentAccessVM } from "@sparkops/contracts/filesystem";
import { BffError } from "@sparkops/contracts";
import {
  fetchFilePreviewBlob,
  filePreviewAccessMessage,
  getFileContentAccess,
  mapFilePreviewError,
} from "@sparkops/web-sdk/filesystem";

type PreviewState =
  | { phase: "idle" }
  | { phase: "loading" }
  | { phase: "unsupported"; message: string }
  | { phase: "error"; message: string }
  | {
      phase: "ready";
      kind: FileContentAccessVM["preview_kind"];
      mimeType: string;
      text?: string;
      objectUrl?: string;
    };

export function FilePreviewPane({
  workspaceId,
  nodeId,
  namespaceId,
  enabled,
}: {
  workspaceId: string;
  nodeId: string;
  namespaceId?: string;
  enabled: boolean;
}): React.ReactElement {
  const [state, setState] = useState<PreviewState>({ phase: "idle" });
  const objectUrlRef = useRef<string | null>(null);

  useEffect(() => {
    if (!enabled) {
      setState({ phase: "idle" });
      return;
    }
    const controller = new AbortController();
    setState({ phase: "loading" });
    if (objectUrlRef.current) {
      URL.revokeObjectURL(objectUrlRef.current);
      objectUrlRef.current = null;
    }

    void getFileContentAccess(
      workspaceId,
      nodeId,
      { namespace_id: namespaceId },
      { signal: controller.signal },
    )
      .then(async (access) => {
        if (controller.signal.aborted) return;
        if (!access.preview_supported) {
          setState({
            phase: "unsupported",
            message: filePreviewAccessMessage(access),
          });
          return;
        }
        const preview = await fetchFilePreviewBlob(
          workspaceId,
          nodeId,
          { namespace_id: namespaceId },
          { signal: controller.signal },
        );
        if (controller.signal.aborted) return;
        const mimeType = preview.mimeType || preview.blob.type || "application/octet-stream";
        const kind = preview.previewKind as FileContentAccessVM["preview_kind"];
        if (kind === "text") {
          const text = await preview.blob.text();
          setState({ phase: "ready", kind: "text", mimeType, text });
          return;
        }
        const objectUrl = URL.createObjectURL(preview.blob);
        objectUrlRef.current = objectUrl;
        setState({
          phase: "ready",
          kind,
          mimeType,
          objectUrl,
        });
      })
      .catch((err: unknown) => {
        if (controller.signal.aborted) return;
        if (err instanceof BffError && err.status === 403) {
          setState({ phase: "error", message: "无权预览该文件内容。" });
          return;
        }
        setState({
          phase: "error",
          message: mapFilePreviewError(err),
        });
      });

    return () => {
      controller.abort();
      if (objectUrlRef.current) {
        URL.revokeObjectURL(objectUrlRef.current);
        objectUrlRef.current = null;
      }
    };
  }, [enabled, namespaceId, nodeId, workspaceId]);

  if (state.phase === "idle" || state.phase === "loading") {
    return createElement(
      "div",
      { className: "empty-state", role: state.phase === "loading" ? "status" : undefined },
      state.phase === "loading" ? "正在加载预览…" : "预览未就绪",
    );
  }
  if (state.phase === "unsupported" || state.phase === "error") {
    return createElement("div", { className: "empty-state", role: "alert" }, state.message);
  }
  if (state.kind === "text" && state.text !== undefined) {
    return createElement(
      "pre",
      {
        className:
          "max-h-96 overflow-auto rounded-md border border-border bg-muted/40 p-4 text-xs leading-relaxed",
      },
      state.text,
    );
  }
  if (state.kind === "image" && state.objectUrl) {
    return createElement("img", {
      src: state.objectUrl,
      alt: "文件预览",
      className: "max-h-96 w-full rounded-md border border-border object-contain",
    });
  }
  if (state.kind === "pdf" && state.objectUrl) {
    return createElement("iframe", {
      src: state.objectUrl,
      title: "PDF 预览",
      className: "h-96 w-full rounded-md border border-border bg-white",
    });
  }
  return createElement("div", { className: "empty-state" }, "无法渲染预览。");
}