"use client";

import { usePathname } from "next/navigation";
import { useEffect, useRef } from "react";
import { recordRecentVisit } from "@/components/layout/command-search-recent";
import { resolveRouteVisitMeta } from "@/components/layout/route-labels";

export function RouteVisitRecorder() {
  const pathname = usePathname();
  const lastPath = useRef<string | null>(null);

  useEffect(() => {
    if (!pathname || pathname === lastPath.current) return;
    if (pathname.startsWith("/login") || pathname.startsWith("/register")) return;
    lastPath.current = pathname;
    const meta = resolveRouteVisitMeta(pathname);
    recordRecentVisit({
      href: pathname,
      label: meta.label,
      group: meta.group,
    });
  }, [pathname]);

  return null;
}
