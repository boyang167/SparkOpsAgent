"use client";

import { createElement } from "react";
import type { ChangeEvent } from "react";

interface UploadPanelProps {
  onFilesSelected: (files: File[]) => void;
  disabled?: boolean;
  emphasis?: "primary" | "secondary";
  size?: "md" | "sm";
}

/** 原生 label 触发文件选择，避免 display:none 的 input 在部分环境下无法被 programmatic click 打开。 */
export function UploadPanel({
  onFilesSelected,
  disabled = false,
  emphasis = "primary",
  size = "md",
}: UploadPanelProps) {
  const buttonClass = `btn btn-${emphasis}${size === "sm" ? " btn-sm" : ""} filesystem-touch-target filesystem-upload-trigger`;

  function handleFiles(files: FileList | null) {
    const selected = Array.from(files ?? []);
    if (selected.length > 0) onFilesSelected(selected);
  }

  if (disabled) {
    return createElement(
      "button",
      { type: "button", className: buttonClass, disabled: true },
      "上传文件",
    );
  }

  return createElement(
    "label",
    { className: buttonClass },
    "上传文件",
    createElement("input", {
      type: "file",
      multiple: true,
      className: "sr-only",
      "aria-label": "上传文件",
      onChange: (event: ChangeEvent<HTMLInputElement>) => {
        handleFiles(event.currentTarget.files);
        event.currentTarget.value = "";
      },
    }),
  );
}