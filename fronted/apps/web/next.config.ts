import type { NextConfig } from "next";

// Windows 本机构建无法创建 standalone 所需的 symlink（EPERM）。
// Docker / Linux CI 仍使用 standalone；本机可用 NEXT_OUTPUT_STANDALONE=1 强制开启。
const useStandalone =
  process.env.NEXT_OUTPUT_STANDALONE === "1" || process.platform !== "win32";

const nextConfig: NextConfig = {
  ...(useStandalone ? { output: "standalone" as const } : {}),
  transpilePackages: [
    "@sparkops/contracts",
    "@sparkops/bff-core",
    "@sparkops/bff-modules",
    "@sparkops/upstream",
    "@sparkops/web-sdk",
  ],
};

export default nextConfig;
