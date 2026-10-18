"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";
import { useEffect, useState } from "react";
import type { MenuTreeVM } from "@sparkops/contracts/governance";
import { governance } from "@sparkops/web-sdk";
import { useShellContext } from "@/components/layout/shell-context";
import {
  kubernetesPathMatches,
  visibleSectionForPath,
} from "./kubernetes-section-nav";

function collectVisiblePaths(tree: MenuTreeVM[]): Set<string> {
  const paths = new Set<string>();
  const visit = (nodes: MenuTreeVM[]) => {
    for (const node of nodes) {
      if (node.hidden) continue;
      const path = node.path.trim();
      if (path.startsWith("/")) paths.add(path);
      visit(node.children ?? []);
    }
  };
  visit(tree);
  return paths;
}

export function KubernetesSectionNav() {
  const pathname = usePathname();
  const { workspaceId } = useShellContext();
  const [visiblePaths, setVisiblePaths] = useState<Set<string> | null>(null);

  useEffect(() => {
    setVisiblePaths(null);
    if (!workspaceId) return;
    let cancelled = false;
    void governance.listNavMenus(workspaceId)
      .then((tree) => {
        if (!cancelled) setVisiblePaths(collectVisiblePaths(tree));
      })
      .catch(() => {
        if (!cancelled) setVisiblePaths(new Set());
      });
    return () => {
      cancelled = true;
    };
  }, [workspaceId]);

  const section = visiblePaths
    ? visibleSectionForPath(pathname, visiblePaths)
    : undefined;

  if (!section || section.items.length <= 1) return null;

  return (
    <nav className="k8s-tabs k8s-section-nav" aria-label={`${section.label}功能`}>
      {section.items.map((item) => {
        const active = kubernetesPathMatches(pathname, item.href);
        return (
          <Link
            key={item.href}
            href={item.href}
            className={`k8s-tab${active ? " k8s-tab-active" : ""}`}
            aria-current={active ? "page" : undefined}
          >
            {item.label}
          </Link>
        );
      })}
    </nav>
  );
}
