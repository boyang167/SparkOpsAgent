"use client";

import { ThemeProvider } from "@/components/layout/theme/ThemeProvider";

export function PlatformShell({ children }: { children: React.ReactNode }) {
  return <ThemeProvider>{children}</ThemeProvider>;
}
