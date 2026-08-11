export type ThemeMode = "light" | "dark" | "system";

export type CardStyle = "border" | "shadow";

export interface ThemeConfig {
  mode: ThemeMode;
  primaryColor: string;
  sidebarDark: boolean;
  borderRadius: number;
  cardStyle: CardStyle;
  showTabIcons: boolean;
}

export const THEME_STORAGE_KEY = "sparkops-theme-config";

export const DEFAULT_THEME_CONFIG: ThemeConfig = {
  mode: "system",
  primaryColor: "#2264f2",
  sidebarDark: false,
  borderRadius: 0.5,
  cardStyle: "border",
  showTabIcons: true,
};

export const PRIMARY_PRESETS = [
  { label: "默认蓝", value: "#2264f2" },
  { label: "紫", value: "#722ed1" },
  { label: "天蓝", value: "#1890ff" },
  { label: "绿", value: "#13c2c2" },
  { label: "青", value: "#2f54eb" },
  { label: "橙", value: "#fa8c16" },
  { label: "粉", value: "#eb2f96" },
] as const;

export const THEME_PRESETS: Array<{ name: string; config: Partial<ThemeConfig> }> = [
  { name: "默认", config: {} },
  { name: "深色侧边栏", config: { sidebarDark: true, primaryColor: "#2264f2" } },
  { name: "紧凑圆角", config: { borderRadius: 0.25, cardStyle: "border" } },
  { name: "阴影卡片", config: { borderRadius: 0.75, cardStyle: "shadow" } },
];

function mixHex(hex: string, amount: number) {
  const raw = hex.replace("#", "");
  const num = parseInt(raw.length === 3 ? raw.split("").map((c) => c + c).join("") : raw, 16);
  const r = (num >> 16) & 255;
  const g = (num >> 8) & 255;
  const b = num & 255;
  const mix = (channel: number) => Math.round(channel + (255 - channel) * amount);
  return `#${[mix(r), mix(g), mix(b)].map((v) => v.toString(16).padStart(2, "0")).join("")}`;
}

export function applyThemeConfig(config: ThemeConfig) {
  if (typeof document === "undefined") return;
  const root = document.documentElement;
  const resolvedMode =
    config.mode === "system"
      ? window.matchMedia("(prefers-color-scheme: dark)").matches
        ? "dark"
        : "light"
      : config.mode;

  root.dataset.theme = resolvedMode;
  root.dataset.sidebarDark = config.sidebarDark ? "true" : "false";
  root.dataset.cardStyle = config.cardStyle;
  root.style.setProperty("--primary", config.primaryColor);
  root.style.setProperty("--primary-hover", mixHex(config.primaryColor, -0.12));
  root.style.setProperty("--primary-soft", mixHex(config.primaryColor, 0.88));
  root.style.setProperty("--accent", config.primaryColor);
  root.style.setProperty("--accent-soft", mixHex(config.primaryColor, 0.88));
  root.style.setProperty("--radius-sm", `${Math.max(0, config.borderRadius * 0.75)}rem`);
  root.style.setProperty("--radius-md", `${Math.max(0, config.borderRadius)}rem`);
  root.style.setProperty("--radius-lg", `${Math.max(0, config.borderRadius * 1.25)}rem`);
  root.style.setProperty("--radius-xl", `${Math.max(0, config.borderRadius * 1.75)}rem`);
}

export function loadThemeConfig(): ThemeConfig {
  if (typeof window === "undefined") return DEFAULT_THEME_CONFIG;
  try {
    const raw = window.localStorage.getItem(THEME_STORAGE_KEY);
    if (!raw) return DEFAULT_THEME_CONFIG;
    return { ...DEFAULT_THEME_CONFIG, ...JSON.parse(raw) };
  } catch {
    return DEFAULT_THEME_CONFIG;
  }
}

export function saveThemeConfig(config: ThemeConfig) {
  if (typeof window === "undefined") return;
  window.localStorage.setItem(THEME_STORAGE_KEY, JSON.stringify(config));
}
