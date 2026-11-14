"use client";

import { createElement, useRef } from "react";
import type { ChangeEvent } from "react";

interface UploadPanelProps {
  onFilesSelected: (files: File[]) => void;
  disabled?: boolean;
  emphasis?: "primary" | "secondary";
}

export function UploadPanel({
  onFilesSelected,
  disabled = false,
  emphasis = "primary",
}: UploadPanelProps) {
  const inputRef = useRef<HTMLInputElement>(null);

  function handleFiles(files: FileList | null) {
    const selected = Array.from(files ?? []);
    if (selected.length > 0) onFilesSelected(selected);
    if (inputRef.current) inputRef.current.value = "";
  }

  return createElement(
    "div",
    null,
    createElement("input", {
      ref: inputRef,
      type: "file",
      multiple: true,
      className: "hidden",
      disabled,
      onChange: (event: ChangeEvent<HTMLInputElement>) => handleFiles(event.target.files),
    }),
    createElement(
      "button",
      {
        type: "button",
        className: `btn btn-${emphasis}`,
        disabled,
        onClick: () => inputRef.current?.click(),
      },
      "上传文件",
    ),
  );
}
