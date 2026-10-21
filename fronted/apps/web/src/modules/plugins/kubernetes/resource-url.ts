import { CATALOG, proxyPath, type ResourceDef } from "./catalog";

type ResourceCollectionOptions = {
  namespace?: string;
  page: number;
  pageSize: number;
  keyword?: string;
  allowAllNamespaces?: boolean;
};

const eventDef = CATALOG.find((item) => item.kind === "Event");

export function resourceCollectionUrl(
  clusterId: string,
  def: ResourceDef,
  options: ResourceCollectionOptions,
): string | null {
  const namespace = options.namespace?.trim();
  if (def.namespaced && !namespace && !options.allowAllNamespaces) return null;
  const params = new URLSearchParams();
  params.set("page", String(options.page));
  params.set("pageSize", String(options.pageSize));
  const keyword = options.keyword?.trim();
  if (keyword) params.set("keyword", keyword);
  return `${proxyPath(clusterId, def, namespace)}?${params.toString()}`;
}

export function podEventsUrl(clusterId: string, namespace: string, podName: string): string | null {
  const ns = namespace.trim();
  const name = podName.trim();
  if (!eventDef || !ns || !name) return null;
  const params = new URLSearchParams({ fieldSelector: `involvedObject.name=${name}` });
  return `${proxyPath(clusterId, eventDef, ns)}?${params.toString()}`;
}
