"use client";

import { createElement, useState } from "react";
import { LifecyclePolicyPanel } from "./LifecyclePolicyPanel";
import { StorageBindingPanel } from "./StorageBindingPanel";
import { StoragePoolPanel } from "./StoragePoolPanel";

export type GovernanceSection = "providers" | "bindings" | "lifecycle";

export type GovernanceWorkspaceProps = {
  workspaceId: string;
  permissions: string[];
};

const sections: Array<{ id: GovernanceSection; label: string }> = [
  { id: "providers", label: "Provider 与存储池" },
  { id: "bindings", label: "作用域绑定" },
  { id: "lifecycle", label: "生命周期策略" },
];

export function GovernanceWorkspace({
  workspaceId,
  permissions,
}: GovernanceWorkspaceProps) {
  const [section, setSection] = useState<GovernanceSection>("providers");

  if (!permissions.includes("storage:read")) {
    return createElement("div", { className: "empty-state" }, "无权查看存储治理。");
  }

  const activePanel =
    section === "providers"
      ? createElement(StoragePoolPanel, { workspaceId, permissions })
      : section === "bindings"
        ? createElement(StorageBindingPanel, { workspaceId, permissions })
        : createElement(LifecyclePolicyPanel, { workspaceId, permissions });

  return createElement(
    "section",
    { className: "stack-lg" },
    createElement(
      "div",
      {
        role: "tablist",
        "aria-label": "存储治理",
        className: "flex max-w-full flex-wrap gap-2 overflow-x-auto",
      },
      sections.map((item) =>
        createElement(
          "button",
          {
            key: item.id,
            type: "button",
            role: "tab",
            "aria-selected": section === item.id,
            className:
              section === item.id ? "btn btn-secondary btn-sm" : "btn btn-ghost btn-sm",
            onClick: () => setSection(item.id),
          },
          item.label,
        ),
      ),
    ),
    createElement("div", { role: "tabpanel" }, activePanel),
  );
}
