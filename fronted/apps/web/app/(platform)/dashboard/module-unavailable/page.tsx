import Link from "next/link";
import { FEATURE_MODULE_LABELS } from "@sparkops/contracts/platform";
import { PageHeader } from "@/components/ui/PageHeader";
import type { EntitlementKey } from "@/components/layout/entitlements";
import { loadConsoleAccess } from "@/server/loaders/console-access";

const MODULE_HINTS: Partial<Record<EntitlementKey, string>> = {
  k8s: "请先在「平台运营 → 集群管理」接入 Kubernetes 集群，或在「平台配置」中启用 K8s 模块。",
  delivery: "当前工作区未开通持续交付能力，请联系管理员在平台配置中启用。",
  observability: "可观测性模块未启用，请联系平台管理员。",
  knowledge: "知识库模块未启用，请联系平台管理员。",
  agent: "智能体 / 项目模块未启用，请联系平台管理员。",
  mcp: "MCP 集成未启用，请联系平台管理员。",
};

export default async function ModuleUnavailablePage({
  searchParams,
}: {
  searchParams: Promise<{ module?: string; from?: string }>;
}) {
  const params = await searchParams;
  const moduleKey = params.module as EntitlementKey | undefined;
  const from = params.from ?? "";
  const label =
    moduleKey && moduleKey in FEATURE_MODULE_LABELS
      ? FEATURE_MODULE_LABELS[moduleKey]
      : "该功能模块";
  const hint = moduleKey ? MODULE_HINTS[moduleKey] : undefined;

  const { isSuperAdmin } = await loadConsoleAccess();

  return (
    <div className="stack-lg">
      <PageHeader
        eyebrow="访问受限"
        title={`${label} 暂不可用`}
        description="当前工作区未开通此能力，或平台尚未完成相关基础设施接入。"
      />

      <div className="card stack-md max-w-xl">
        {from ? (
          <p className="text-sm text-muted">
            请求路径：<code className="font-mono text-xs">{from}</code>
          </p>
        ) : null}
        {hint ? <p className="text-sm">{hint}</p> : null}
        <div className="flex flex-wrap gap-3">
          <Link href="/dashboard/workplace" className="btn">
            返回工作台
          </Link>
          {isSuperAdmin ? (
            <Link href="/platform/settings" className="btn-secondary">
              打开平台配置
            </Link>
          ) : null}
          {moduleKey === "k8s" && isSuperAdmin ? (
            <Link href="/platform/clusters" className="btn-secondary">
              管理集群
            </Link>
          ) : null}
        </div>
      </div>
    </div>
  );
}
