"use client";

import {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useMemo,
  useState,
} from "react";
import {
  applyThemeConfig,
  DEFAULT_THEME_CONFIG,
  loadThemeConfig,
  saveThemeConfig,
  type ThemeConfig,
  type ThemeMode,
} from "./theme-config";

interface ThemeContextValue {
  config: ThemeConfig;
  setMode: (mode: ThemeMode) => void;
  patchConfig: (patch: Partial<ThemeConfig>) => void;
  resetConfig: () => void;
  toggleDarkLight: () => void;
}

const ThemeContext = createContext<ThemeContextValue | null>(null);

export function ThemeProvider({ children }: { children: React.ReactNode }) {
  const [config, setConfig] = useState<ThemeConfig>(DEFAULT_THEME_CONFIG);
  const [ready, setReady] = useState(false);

  useEffect(() => {
    const loaded = loadThemeConfig();
    setConfig(loaded);
    applyThemeConfig(loaded);
    setReady(true);
  }, []);

  useEffect(() => {
    if (!ready) return;
    applyThemeConfig(config);
    saveThemeConfig(config);
  }, [config, ready]);

  useEffect(() => {
    if (!ready || config.mode !== "system") return;
    const media = window.matchMedia("(prefers-color-scheme: dark)");
    const onChange = () => applyThemeConfig(config);
    media.addEventListener("change", onChange);
    return () => media.removeEventListener("change", onChange);
  }, [config, ready]);

  const patchConfig = useCallback((patch: Partial<ThemeConfig>) => {
    setConfig((prev) => ({ ...prev, ...patch }));
  }, []);

  const setMode = useCallback((mode: ThemeMode) => {
    patchConfig({ mode });
  }, [patchConfig]);

  const resetConfig = useCallback(() => {
    setConfig(DEFAULT_THEME_CONFIG);
  }, []);

  const toggleDarkLight = useCallback(() => {
    setConfig((prev) => {
      const current =
        prev.mode === "system"
          ? window.matchMedia("(prefers-color-scheme: dark)").matches
            ? "dark"
            : "light"
          : prev.mode;
      return { ...prev, mode: current === "dark" ? "light" : "dark" };
    });
  }, []);

  const value = useMemo(
    () => ({ config, setMode, patchConfig, resetConfig, toggleDarkLight }),
    [config, setMode, patchConfig, resetConfig, toggleDarkLight],
  );

  return <ThemeContext.Provider value={value}>{children}</ThemeContext.Provider>;
}

export function useThemeConfig() {
  const ctx = useContext(ThemeContext);
  if (!ctx) {
    throw new Error("useThemeConfig must be used within ThemeProvider");
  }
  return ctx;
}

export function useResolvedThemeMode(): "light" | "dark" {
  const { config } = useThemeConfig();
  const [mode, setMode] = useState<"light" | "dark">("light");

  useEffect(() => {
    const resolve = () => {
      if (config.mode === "dark") return "dark";
      if (config.mode === "light") return "light";
      return window.matchMedia("(prefers-color-scheme: dark)").matches ? "dark" : "light";
    };
    setMode(resolve());
    if (config.mode !== "system") return;
    const media = window.matchMedia("(prefers-color-scheme: dark)");
    const onChange = () => setMode(resolve());
    media.addEventListener("change", onChange);
    return () => media.removeEventListener("change", onChange);
  }, [config.mode]);

  return mode;
}
