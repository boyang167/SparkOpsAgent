export const VELAUX_IFRAME_SANDBOX = "allow-scripts allow-forms allow-popups allow-downloads";

export function buildVelauxFrameProps(clusterId: string) {
  return {
    src: `/api/bff/v1/plugins/kubernetes/clusters/${encodeURIComponent(clusterId)}/velaux/`,
    sandbox: VELAUX_IFRAME_SANDBOX,
    referrerPolicy: "no-referrer",
  } as const;
}
