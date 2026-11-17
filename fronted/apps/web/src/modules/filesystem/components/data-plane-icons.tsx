// Inline SVG icons for data plane module navigation.

import { createElement } from "react";

const iconProps = {
  width: 16,
  height: 16,
  viewBox: "0 0 24 24",
  fill: "none",
  stroke: "currentColor",
  strokeWidth: 1.75,
  strokeLinecap: "round" as const,
  strokeLinejoin: "round" as const,
  "aria-hidden": true,
};

export function DataPlaneBrowseIcon() {
  return createElement(
    "svg",
    iconProps,
    createElement("path", { d: "M4 6h16v12H4z" }),
    createElement("path", { d: "M4 10h16" }),
  );
}

export function DataPlaneTrashIcon() {
  return createElement(
    "svg",
    iconProps,
    createElement("path", { d: "M4 7h16" }),
    createElement("path", { d: "M9 7V5h6v2" }),
    createElement("path", { d: "M10 11v6M14 11v6" }),
    createElement("path", { d: "M7 7l1 12h8l1-12" }),
  );
}

export function DataPlaneTransferIcon() {
  return createElement(
    "svg",
    iconProps,
    createElement("path", { d: "M7 17 12 22l5-5" }),
    createElement("path", { d: "M12 2v20" }),
    createElement("path", { d: "M7 7 12 2l5 5" }),
  );
}

export function DataPlaneFolderIcon() {
  return createElement(
    "svg",
    { ...iconProps, width: 18, height: 18 },
    createElement("path", { d: "M4 8h6l2 2h8v10H4z" }),
  );
}

export function DataPlaneFileIcon() {
  return createElement(
    "svg",
    { ...iconProps, width: 18, height: 18 },
    createElement("path", { d: "M8 4h8l4 4v12H8z" }),
    createElement("path", { d: "M16 4v4h4" }),
  );
}

export function DataPlanePencilIcon() {
  return createElement(
    "svg",
    { ...iconProps, width: 15, height: 15 },
    createElement("path", { d: "M12 20h9" }),
    createElement("path", { d: "m15 5 4 4-9 9-4 1 1-4z" }),
  );
}

export function DataPlaneMoreIcon() {
  return createElement(
    "svg",
    { ...iconProps, width: 15, height: 15 },
    createElement("circle", { cx: 5, cy: 12, r: 1.25, fill: "currentColor", stroke: "none" }),
    createElement("circle", { cx: 12, cy: 12, r: 1.25, fill: "currentColor", stroke: "none" }),
    createElement("circle", { cx: 19, cy: 12, r: 1.25, fill: "currentColor", stroke: "none" }),
  );
}

export function DataPlanePlusFolderIcon() {
  return createElement(
    "svg",
    { ...iconProps, width: 14, height: 14 },
    createElement("path", { d: "M12 5v14M5 12h14" }),
  );
}

export function DataPlaneSearchIcon() {
  return createElement(
    "svg",
    { ...iconProps, width: 14, height: 14 },
    createElement("circle", { cx: 11, cy: 11, r: 7 }),
    createElement("path", { d: "M20 20l-4-4" }),
  );
}

export function DataPlaneUploadIcon() {
  return createElement(
    "svg",
    { ...iconProps, width: 14, height: 14 },
    createElement("path", { d: "M12 16V4" }),
    createElement("path", { d: "m7 9 5-5 5 5" }),
    createElement("path", { d: "M5 20h14" }),
  );
}