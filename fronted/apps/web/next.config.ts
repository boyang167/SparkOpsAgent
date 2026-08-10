import type { NextConfig } from "next";

const nextConfig: NextConfig = {
  output: "standalone",
  transpilePackages: [
    "@sparkops/contracts",
    "@sparkops/bff-core",
    "@sparkops/bff-modules",
    "@sparkops/upstream",
    "@sparkops/web-sdk",
  ],
};

export default nextConfig;
