"use client";

import { usePathname, useRouter } from "next/navigation";
import { createElement, useEffect, useState } from "react";
import { LifecyclePolicyPanel } from "./LifecyclePolicyPanel";
import { StorageBindingPanel } from "./StorageBindingPanel";
import { StoragePoolPanel } from "./StoragePoolPanel";

export type GovernanceSection = "providers" | "pools" | "bindings" | "lifecycle";

export type GovernanceWorkspaceProps = {
  workspaceId: string;
  permissions: string[];
  /** 与控制台路由同步的当前 Tab（仍展示四 Tab 条） */
  activeSection?: GovernanceSection;
};

const sections: Array<{ id: GovernanceSection; label: string }> = [
  { id: "providers", label: "Storage Providers" },
  { id: "pools", label: "存储池" },
  { id: "bindings", label: "作用域绑定" },
  { id: "lifecycle", label: "生命周期策略" },
];

export const GOVERNANCE_SECTION_HREFS: Record<GovernanceSection, string> = {
  providers: "/files/control/providers",
  pools: "/files/control/pools",
  bindings: "/files/control/bindings",
  lifecycle: "/files/governance/lifecycle",
};

export function governanceSectionFromPathname(pathname: string): GovernanceSection | undefined {
  if (pathname.startsWith("/files/control/providers")) return "providers";
  if (pathname.startsWith("/files/control/pools")) return "pools";
  if (pathname.startsWith("/files/control/bindings")) return "bindings";
  if (pathname.startsWith("/files/governance/lifecycle")) return "lifecycle";
  return undefined;
}

function renderSection(
  section: GovernanceSection,
  workspaceId: string,
  permissions: string[],
) {
  switch (section) {
    case "providers":
      return createElement(StoragePoolPanel, {
        workspaceId,
        permissions,
        focus: "providers",
      });
    case "pools":
      return createElement(StoragePoolPanel, {
        workspaceId,
        permissions,
        focus: "pools",
      });
    case "bindings":
      return createElement(StorageBindingPanel, { workspaceId, permissions });
    case "lifecycle":
      return createElement(LifecyclePolicyPanel, { workspaceId, permissions });
    default:
      return null;
  }
}

export function GovernanceWorkspace({
  workspaceId,
  permissions,
  activeSection,
}: GovernanceWorkspaceProps) {
  const router = useRouter();
  const pathname = usePathname();
  const routeSection = governanceSectionFromPathname(pathname);
  const [section, setSection] = useState<GovernanceSection>(
    activeSection ?? routeSection ?? "providers",
  );

  useEffect(() => {
    const next = activeSection ?? routeSection;
    if (next) setSection(next);
  }, [activeSection, routeSection]);

  if (!permissions.includes("storage:read")) {
    return createElement("div", { className: "empty-state" }, "无权查看存储治理。");
  }

  const activePanel = renderSection(section, workspaceId, permissions);

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
              section === item.id
                ? "btn btn-secondary btn-sm filesystem-touch-target"
                : "btn btn-ghost btn-sm filesystem-touch-target",
            onClick: () => {
              if (item.id === section) return;
              const href = GOVERNANCE_SECTION_HREFS[item.id];
              if (governanceSectionFromPathname(pathname) || pathname.startsWith("/files/control/")) {
                router.push(href);
                return;
              }
              setSection(item.id);
            },
          },
          item.label,
        ),
      ),
    ),
    createElement("div", { role: "tabpanel" }, activePanel),
  );
}