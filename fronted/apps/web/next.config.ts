import type { NextConfig } from "next";

// Windows 本机构建无法创建 standalone 所需的 symlink（EPERM）。
// Docker / Linux CI 仍使用 standalone；本机可用 NEXT_OUTPUT_STANDALONE=1 强制开启。
const useStandalone =
  process.env.NEXT_OUTPUT_STANDALONE === "1" || process.platform !== "win32";

const legacyRedirects = [
  { source: "/permission/roles", destination: "/admin/roles", permanent: false },
  { source: "/permission/members", destination: "/admin/members", permanent: false },
  { source: "/permission/menus", destination: "/admin/menus", permanent: false },
  { source: "/permission/apis", destination: "/admin/apis", permanent: false },
  { source: "/permission/data", destination: "/admin/data-policies", permanent: false },
  { source: "/permission/api-tokens", destination: "/admin/api-tokens", permanent: false },
  { source: "/proclaims", destination: "/admin/announcements", permanent: false },
  { source: "/tools", destination: "/admin/tools", permanent: false },
  { source: "/monitor/operation", destination: "/admin/operation-logs", permanent: false },
  { source: "/monitor/org-audit", destination: "/admin/org-audit", permanent: false },
  { source: "/monitor/server", destination: "/admin/server-status", permanent: false },
  { source: "/settings/platform/members", destination: "/platform/accounts", permanent: false },
  { source: "/settings/platform/orgs", destination: "/platform/organizations", permanent: false },
  { source: "/cluster/list", destination: "/platform/clusters", permanent: false },
  { source: "/cluster/sync", destination: "/platform/clusters/sync", permanent: false },
  { source: "/monitor/platform-audit", destination: "/platform/audit", permanent: false },
  { source: "/templates", destination: "/market/dockerfiles", permanent: false },
];

const nextConfig: NextConfig = {
  ...(useStandalone ? { output: "standalone" as const } : {}),
  transpilePackages: [
    "@sparkops/contracts",
    "@sparkops/bff-core",
    "@sparkops/bff-modules",
    "@sparkops/upstream",
    "@sparkops/web-sdk",
  ],
  async redirects() {
    return legacyRedirects;
  },
};

export default nextConfig;
