"use client";

import { useRef, useState } from "react";
import {
  createTransferSession,
  uploadTransfer,
  type TransferProgress,
} from "@sparkops/web-sdk/filesystem";
import { TransferProgress as TransferProgressBar } from "./TransferProgress";
import { getUploadGate } from "./upload-panel-gate";

interface UploadPanelProps {
  workspaceId: string;
  parentId: string;
  providerId?: string;
  namespaceId?: string;
  onUploaded?: () => void;
}

export function UploadPanel({
  workspaceId,
  parentId,
  providerId,
  namespaceId,
  onUploaded,
}: UploadPanelProps) {
  const inputRef = useRef<HTMLInputElement>(null);
  const [busy, setBusy] = useState(false);
  const [progress, setProgress] = useState<TransferProgress | null>(null);
  const [error, setError] = useState<string | null>(null);

  const { uploadEnabled, helperText } = getUploadGate(providerId);

  async function handleFiles(files: FileList | null) {
    const file = files?.[0];
    if (!file) return;
    if (!uploadEnabled || !providerId) {
      return;
    }
    setBusy(true);
    setError(null);
    setProgress(null);
    try {
      const session = await createTransferSession(workspaceId, {
        parent_id: parentId,
        namespace_id: namespaceId,
        name: file.name,
        protocol: "DIRECT",
        size: file.size,
        provider_id: providerId,
        classification: "INTERNAL",
      });
      await uploadTransfer(file, {
        workspaceId,
        session,
        onProgress: setProgress,
      });
      onUploaded?.();
    } catch (err) {
      setError(err instanceof Error ? err.message : "上传失败");
    } finally {
      setBusy(false);
      setProgress(null);
      if (inputRef.current) inputRef.current.value = "";
    }
  }

  return (
    <div className="stack-sm">
      <input
        ref={inputRef}
        type="file"
        className="hidden"
        disabled={busy || !uploadEnabled}
        onChange={(event) => void handleFiles(event.target.files)}
      />
      <button
        type="button"
        className="btn btn-secondary"
        disabled={busy || !uploadEnabled}
        aria-disabled={!uploadEnabled}
        onClick={() => {
          if (!uploadEnabled) return;
          inputRef.current?.click();
        }}
      >
        {busy ? "上传中…" : "上传文件"}
      </button>
      {helperText ? <p className="text-sm text-muted">{helperText}</p> : null}
      {progress ? <TransferProgressBar progress={progress} /> : null}
      {error ? <p className="text-sm text-danger">{error}</p> : null}
    </div>
  );
}
