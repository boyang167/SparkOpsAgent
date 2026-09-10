"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";

interface ProjectNavProps {
  projectId: string;
  projectName: string;
}

const PROJECT_NAV_ITEMS = [
  { segment: "", label: "项目概览", suffix: "" },
  { segment: "agents", label: "智能体", suffix: "/agents" },
  { segment: "workflows", label: "工作流", suffix: "/workflows" },
  { segment: "runtime", label: "运行环境", suffix: "/runtime" },
  { segment: "config", label: "配置", suffix: "/config" },
] as const;

const PROJECT_NAV_SUFFIXES = PROJECT_NAV_ITEMS.map((item) => item.suffix).filter(Boolean);

function isNavActive(pathname: string, projectId: string, suffix: string) {
  const base = `/projects/${projectId}`;
  if (suffix === "") {
    return (
      pathname === base ||
      (pathname.startsWith(`${base}/`) &&
        !PROJECT_NAV_SUFFIXES.some((navSuffix) => pathname.startsWith(`${base}${navSuffix}`)))
    );
  }
  return pathname === `${base}${suffix}` || pathname.startsWith(`${base}${suffix}/`);
}

export function ProjectNav({ projectId, projectName }: ProjectNavProps) {
  const pathname = usePathname();
  const base = `/projects/${projectId}`;

  return (
    <>
      <Link href="/projects" className="nav-back-link">
        ← 全部项目
      </Link>

      <div className="nav-project-head">
        <p className="nav-project-label">当前项目</p>
        <p className="nav-project-name" title={projectName}>
          {projectName}
        </p>
      </div>

      {PROJECT_NAV_ITEMS.map((item) => {
        const href = `${base}${item.suffix}`;
        const active = isNavActive(pathname, projectId, item.suffix);
        return (
          <Link
            key={item.label}
            href={href}
            className={`nav-link${active ? " nav-link-active" : ""}`}
          >
            <span className="nav-icon">
              <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
                <circle cx="12" cy="12" r="2" />
              </svg>
            </span>
            {item.label}
          </Link>
        );
      })}
    </>
  );
}
