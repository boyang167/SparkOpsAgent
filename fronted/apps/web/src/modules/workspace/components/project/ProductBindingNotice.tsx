"use client";

import { useEffect, useState } from "react";
import Link from "next/link";
import { product } from "@sparkops/web-sdk";

interface ProductBindingNoticeProps {
  workspaceId: string;
  productId: string;
  action: string;
}

export function ProductBindingNotice({ workspaceId, productId, action }: ProductBindingNoticeProps) {
  const [productName, setProductName] = useState<string | null>(null);

  useEffect(() => {
    let cancelled = false;
    setProductName(null);
    void product
      .getProduct(workspaceId, productId)
      .then((item) => {
        if (!cancelled) setProductName(item.display_name || item.name);
      })
      .catch(() => undefined);
    return () => {
      cancelled = true;
    };
  }, [workspaceId, productId]);

  return (
    <div className="product-binding-notice" role="alert">
      <p className="product-binding-notice-title">
        项目已绑定产品{productName ? `「${productName}」` : ""}，暂不能{action}
      </p>
      <p className="product-binding-notice-desc">
        请先在产品的「项目配置」中解绑该项目，再回来{action}。
      </p>
      <Link className="product-binding-notice-link" href={`/products/${productId}?tab=projects`}>
        前往产品解绑
      </Link>
    </div>
  );
}
