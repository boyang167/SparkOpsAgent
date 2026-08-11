const iconClass = "menu-icon-svg";

export const MENU_ICON_OPTIONS = [
  { value: "", label: "无图标" },
  { value: "dashboard", label: "仪表盘" },
  { value: "project", label: "项目" },
  { value: "delivery", label: "交付" },
  { value: "infra", label: "基础设施" },
  { value: "resource", label: "资源" },
  { value: "permission", label: "权限" },
  { value: "monitor", label: "监控" },
  { value: "platform", label: "平台" },
  { value: "settings", label: "设置" },
  { value: "default", label: "通用" },
] as const;

export function MenuIconPreview({ icon, className }: { icon?: string; className?: string }) {
  return (
    <span className={`menu-icon-preview ${className ?? ""}`} aria-hidden>
      {renderMenuIcon(icon)}
    </span>
  );
}

export function renderMenuIcon(icon?: string) {
  switch (icon) {
    case "dashboard":
      return (
        <svg className={iconClass} viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
          <rect x="3" y="3" width="7" height="7" rx="1" />
          <rect x="14" y="3" width="7" height="7" rx="1" />
          <rect x="3" y="14" width="7" height="7" rx="1" />
          <rect x="14" y="14" width="7" height="7" rx="1" />
        </svg>
      );
    case "project":
      return (
        <svg className={iconClass} viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
          <path d="M3 7h18M3 12h18M3 17h18" strokeLinecap="round" />
        </svg>
      );
    case "delivery":
      return (
        <svg className={iconClass} viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
          <path d="M12 3v4M12 17v4M3 12h4M17 12h4" strokeLinecap="round" />
          <circle cx="12" cy="12" r="3" />
        </svg>
      );
    case "infra":
      return (
        <svg className={iconClass} viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
          <path d="M4 6h16M4 12h16M4 18h10" strokeLinecap="round" />
        </svg>
      );
    case "resource":
      return (
        <svg className={iconClass} viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
          <path d="M6 6h12v12H6zM9 6V4h6v2M9 18v2h6v-2" strokeLinejoin="round" />
        </svg>
      );
    case "permission":
      return (
        <svg className={iconClass} viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
          <path d="M12 3l7 4v5c0 4.4-3 8.5-7 9-4-.5-7-4.6-7-9V7l7-4z" strokeLinejoin="round" />
        </svg>
      );
    case "monitor":
      return (
        <svg className={iconClass} viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
          <path d="M4 19V5M4 19h16M8 15v-4M12 15V9M16 15v-2" strokeLinecap="round" />
        </svg>
      );
    case "platform":
      return (
        <svg className={iconClass} viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
          <path d="M3 21h18M6 21V7l6-3 6 3v14M9 21v-6h6v6" strokeLinejoin="round" />
        </svg>
      );
    case "settings":
      return (
        <svg className={iconClass} viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
          <circle cx="12" cy="8" r="4" />
          <path d="M4 20c0-4 4-6 8-6s8 2 8 6" strokeLinecap="round" />
        </svg>
      );
    default:
      return (
        <svg className={iconClass} viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
          <circle cx="12" cy="12" r="2" />
          <path d="M12 2v4M12 18v4M2 12h4M18 12h4" strokeLinecap="round" />
        </svg>
      );
  }
}
