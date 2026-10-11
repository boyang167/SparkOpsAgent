export type ThemeMode = "light" | "dark" | "system";

export type CardStyle = "border" | "shadow";

export interface ThemeConfig {
  mode: ThemeMode;
  primaryColor: string;
  sidebarDark: boolean;
  borderRadius: number;
  cardStyle: CardStyle;
  showNavIcons: boolean;
}

export const THEME_STORAGE_KEY = "sparkops-theme-config";

export const DEFAULT_THEME_CONFIG: ThemeConfig = {
  mode: "system",
  primaryColor: "#2264f2",
  sidebarDark: false,
  borderRadius: 1,
  cardStyle: "shadow",
  showNavIcons: true,
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

type PresetConfig = Omit<ThemeConfig, "mode">;

const PRESET_BASE: PresetConfig = {
  primaryColor: DEFAULT_THEME_CONFIG.primaryColor,
  sidebarDark: DEFAULT_THEME_CONFIG.sidebarDark,
  borderRadius: DEFAULT_THEME_CONFIG.borderRadius,
  cardStyle: DEFAULT_THEME_CONFIG.cardStyle,
  showNavIcons: DEFAULT_THEME_CONFIG.showNavIcons,
};

/** Presets are complete (except mode) so switching between them never leaves stale settings behind. */
export const THEME_PRESETS: Array<{ name: string; description: string; config: PresetConfig }> = [
  { name: "默认", description: "柔和圆角、轻阴影卡片", config: PRESET_BASE },
  {
    name: "深色侧边栏",
    description: "侧边栏使用深色背景，内容区保持明亮",
    config: { ...PRESET_BASE, sidebarDark: true },
  },
  {
    name: "紧凑",
    description: "小圆角、细边框卡片，信息更密集",
    config: { ...PRESET_BASE, borderRadius: 0.4, cardStyle: "border" },
  },
  {
    name: "极简",
    description: "隐藏菜单图标，只保留文字",
    config: { ...PRESET_BASE, showNavIcons: false },
  },
];

export function isPresetActive(config: ThemeConfig, preset: PresetConfig) {
  return (
    config.primaryColor.toLowerCase() === preset.primaryColor.toLowerCase() &&
    config.sidebarDark === preset.sidebarDark &&
    config.borderRadius === preset.borderRadius &&
    config.cardStyle === preset.cardStyle &&
    config.showNavIcons === preset.showNavIcons
  );
}

/** amount > 0 mixes toward white, amount < 0 mixes toward black. */
function shadeHex(hex: string, amount: number) {
  const raw = hex.replace("#", "");
  const full = raw.length === 3 ? raw.split("").map((c) => c + c).join("") : raw;
  const num = parseInt(full, 16);
  if (Number.isNaN(num)) return hex;
  const channels = [(num >> 16) & 255, (num >> 8) & 255, num & 255];
  const shaded = channels.map((channel) => {
    const next = amount >= 0 ? channel + (255 - channel) * amount : channel * (1 + amount);
    return Math.min(255, Math.max(0, Math.round(next)));
  });
  return `#${shaded.map((v) => v.toString(16).padStart(2, "0")).join("")}`;
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
  root.dataset.navIcons = config.showNavIcons ? "true" : "false";
  root.style.setProperty("--primary", config.primaryColor);
  root.style.setProperty("--primary-hover", shadeHex(config.primaryColor, -0.12));
  root.style.setProperty("--primary-soft", shadeHex(config.primaryColor, 0.88));
  root.style.setProperty("--accent", config.primaryColor);
  root.style.setProperty("--accent-soft", shadeHex(config.primaryColor, 0.88));
  const radius = Math.max(0, config.borderRadius);
  root.style.setProperty("--radius-sm", `${radius * 0.75}rem`);
  root.style.setProperty("--radius-md", `${radius}rem`);
  root.style.setProperty("--radius-lg", `${radius * 1.25}rem`);
  root.style.setProperty("--radius-xl", `${radius * 1.5}rem`);
}

export function loadThemeConfig(): ThemeConfig {
  if (typeof window === "undefined") return DEFAULT_THEME_CONFIG;
  try {
    const raw = window.localStorage.getItem(THEME_STORAGE_KEY);
    if (!raw) return DEFAULT_THEME_CONFIG;
    const { showTabIcons, ...stored } = JSON.parse(raw) as Partial<ThemeConfig> & {
      showTabIcons?: boolean;
    };
    return {
      ...DEFAULT_THEME_CONFIG,
      ...(typeof showTabIcons === "boolean" ? { showNavIcons: showTabIcons } : {}),
      ...stored,
    };
  } catch {
    return DEFAULT_THEME_CONFIG;
  }
}

export function saveThemeConfig(config: ThemeConfig) {
  if (typeof window === "undefined") return;
  window.localStorage.setItem(THEME_STORAGE_KEY, JSON.stringify(config));
}
