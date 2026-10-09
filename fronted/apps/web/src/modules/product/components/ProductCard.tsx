"use client";

import Link from "next/link";
import { useRouter } from "next/navigation";
import type { ProductOverviewVM, ProductVM } from "@sparkops/contracts/product";
import {
  formatDateTime,
  productAccentColor,
  productDisplayName,
  productStatusLabel,
  productStatusTagClass,
} from "./product-utils";

interface ProductCardProps {
  product: ProductVM;
  overview?: ProductOverviewVM | null;
  overviewLoading?: boolean;
  detailsHref: string;
}

function MetricBlock({ label, value, muted }: { label: string; value: string; muted?: boolean }) {
  return (
    <div className="project-run-metric">
      <span className="project-run-metric-label">{label}</span>
      <strong className={`project-run-metric-value${muted ? " is-muted" : ""}`}>{value}</strong>
    </div>
  );
}

export function ProductCard({
  product,
  overview,
  overviewLoading = false,
  detailsHref,
}: ProductCardProps) {
  const router = useRouter();
  const displayName = productDisplayName(product);
  const showTechnicalName = product.name.trim() !== displayName.trim();
  const accent = productAccentColor(product.name);
  const initial = displayName.charAt(0).toUpperCase() || "P";
  const description = product.description?.trim() || "暂无产品描述";

  function handleCardClick(event: React.MouseEvent<HTMLElement>) {
    if ((event.target as HTMLElement).closest("a, button")) return;
    router.push(detailsHref);
  }

  function handleCardKeyDown(event: React.KeyboardEvent<HTMLElement>) {
    if (event.key !== "Enter" && event.key !== " ") return;
    event.preventDefault();
    router.push(detailsHref);
  }

  const projectCount = overviewLoading ? "—" : String(overview?.project_count ?? 0);
  const environmentCount = overviewLoading ? "—" : String(overview?.environment_count ?? 0);

  return (
    <article
      className="project-overview-card project-overview-card-interactive"
      style={{ ["--project-accent" as string]: accent }}
      onClick={handleCardClick}
      onKeyDown={handleCardKeyDown}
      role="link"
      tabIndex={0}
    >
      <header className="project-overview-header">
        <div className="project-overview-identity">
          <div className="project-lang-icon project-lang-icon-ring" style={{ backgroundColor: accent }}>
            {initial}
          </div>
          <div className="project-overview-heading">
            <div className="project-overview-title-row">
              <h3>{displayName}</h3>
              <span className={productStatusTagClass(product.status)}>{productStatusLabel(product.status)}</span>
            </div>
            {showTechnicalName ? <p className="project-card-subtitle">{product.name}</p> : null}
            <div className="project-overview-meta">
              {product.owner ? <span>{product.owner}</span> : <span>未指定负责人</span>}
            </div>
          </div>
        </div>
      </header>

      <p className={`project-card-desc${description === "暂无产品描述" ? " is-placeholder" : ""}`}>{description}</p>

      <div className="project-overview-body">
        <section className="project-overview-panel" aria-label="版本信息">
          <header className="project-overview-panel-head">
            <h4 className="project-overview-panel-title">版本信息</h4>
          </header>
          <div className="project-run-stats">
            <MetricBlock label="当前版本" value={product.version?.trim() || "—"} muted={!product.version?.trim()} />
            <MetricBlock
              label="下一版本"
              value={product.next_version?.trim() || "—"}
              muted={!product.next_version?.trim()}
            />
          </div>
        </section>

        <section className="project-overview-panel" aria-label="关联概况">
          <header className="project-overview-panel-head">
            <h4 className="project-overview-panel-title">关联概况</h4>
            {overviewLoading ? <span className="project-overview-panel-meta">加载中…</span> : null}
          </header>
          <div className="project-run-stats">
            <MetricBlock label="关联项目" value={projectCount} muted={overviewLoading} />
            <MetricBlock label="运行环境" value={environmentCount} muted={overviewLoading} />
          </div>
        </section>
      </div>

      <footer className="project-workbench-footer">
        <span className="text-muted">创建于 {formatDateTime(product.created_at)}</span>
        <div className="project-workbench-actions">
          <Link href={detailsHref} className="project-action-btn is-config">
            进入产品
          </Link>
        </div>
      </footer>
    </article>
  );
}
