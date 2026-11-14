"use client";

import { createElement, useRef, useState } from "react";
import type { DragEvent, ReactNode } from "react";
import { FILE_NODE_DRAG_TYPE } from "./directory-dnd";

type FileDropZoneProps = {
  targetParentId: string;
  targetName: string;
  canWrite: boolean;
  onFiles: (files: File[], targetParentId: string) => void;
  children?: ReactNode;
};

function dragTypes(dataTransfer: DataTransfer): string[] {
  return Array.from(dataTransfer.types ?? []);
}

function isDesktopFileDrag(dataTransfer: DataTransfer) {
  const types = dragTypes(dataTransfer);
  return types.includes("Files") && !types.includes(FILE_NODE_DRAG_TYPE);
}

function containsDirectory(dataTransfer: DataTransfer) {
  return Array.from(dataTransfer.items ?? []).some((item) => {
    const entry = (
      item as DataTransferItem & {
        webkitGetAsEntry?: () => { isDirectory?: boolean } | null;
      }
    ).webkitGetAsEntry?.();
    return entry?.isDirectory === true;
  });
}

export function FileDropZone({
  targetParentId,
  targetName,
  canWrite,
  onFiles,
  children,
}: FileDropZoneProps) {
  const [active, setActive] = useState(false);
  const [error, setError] = useState<string>();
  const dragDepth = useRef(0);

  function accepts(event: DragEvent<HTMLDivElement>) {
    return canWrite && isDesktopFileDrag(event.dataTransfer);
  }

  return createElement(
    "div",
    {
      "data-testid": "file-drop-zone",
      className: "relative min-w-0 flex-1",
      onDragEnter: (event: DragEvent<HTMLDivElement>) => {
        if (!accepts(event)) return;
        event.preventDefault();
        dragDepth.current += 1;
        setError(undefined);
        setActive(true);
      },
      onDragOver: (event: DragEvent<HTMLDivElement>) => {
        if (!accepts(event)) return;
        event.preventDefault();
        event.dataTransfer.dropEffect = "copy";
      },
      onDragLeave: (event: DragEvent<HTMLDivElement>) => {
        if (!accepts(event)) return;
        dragDepth.current = Math.max(0, dragDepth.current - 1);
        if (dragDepth.current === 0) setActive(false);
      },
      onDrop: (event: DragEvent<HTMLDivElement>) => {
        dragDepth.current = 0;
        setActive(false);
        if (!accepts(event)) return;
        event.preventDefault();
        event.stopPropagation();
        if (containsDirectory(event.dataTransfer)) {
          setError("当前版本不支持目录递归上传");
          return;
        }
        const files = Array.from(event.dataTransfer.files ?? []);
        if (files.length > 0) onFiles(files, targetParentId);
      },
    },
    children,
    active
      ? createElement(
          "div",
          {
            role: "status",
            className:
              "absolute inset-0 z-10 flex items-center justify-center rounded-lg bg-surface/95 text-sm font-medium shadow-sm",
          },
          `释放以上传到“${targetName}”`,
        )
      : null,
    error
      ? createElement(
          "p",
          { role: "alert", className: "mt-2 text-sm text-danger" },
          error,
        )
      : null,
  );
}
