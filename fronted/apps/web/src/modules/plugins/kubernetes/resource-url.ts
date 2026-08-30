import { proxyPath, type ResourceDef } from "./catalog";

type ResourceCollectionOptions = {
  namespace?: string;
  page: number;
  pageSize: number;
  keyword?: string;
};

export function resourceCollectionUrl(
  clusterId: string,
  def: ResourceDef,
  options: ResourceCollectionOptions,
): string | null {
  const namespace = options.namespace?.trim();
  if (def.namespaced && !namespace) return null;
  const params = new URLSearchParams();
  params.set("page", String(options.page));
  params.set("pageSize", String(options.pageSize));
  const keyword = options.keyword?.trim();
  if (keyword) params.set("keyword", keyword);
  return `${proxyPath(clusterId, def, namespace)}?${params.toString()}`;
}
