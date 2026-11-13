import { describe, expect, it } from "vitest";
import { resolveRouteVisitMeta } from "./route-labels";

describe("resolveRouteVisitMeta", () => {
  it("uses the redesigned business navigation labels", () => {
    expect(resolveRouteVisitMeta("/knowledge")).toEqual({
      label: "知识库",
      group: "知识库",
    });
    expect(resolveRouteVisitMeta("/files")).toEqual({
      label: "文件",
      group: "文件",
    });
    expect(resolveRouteVisitMeta("/cronjobs")).toEqual({
      label: "定时构建",
      group: "产品交付",
    });
    expect(resolveRouteVisitMeta("/monitor/log-viewer")).toEqual({
      label: "日志查询",
      group: "运行观测",
    });
  });

  it("labels consolidated kubernetes routes", () => {
    expect(resolveRouteVisitMeta("/plugins/kubernetes/gateway")).toEqual({
      label: "Gateway",
      group: "Kubernetes",
    });
    expect(resolveRouteVisitMeta("/plugins/kubernetes/storages")).toEqual({
      label: "存储",
      group: "Kubernetes",
    });
    expect(resolveRouteVisitMeta("/plugins/kubernetes/cloudtty")).toEqual({
      label: "CloudTTY",
      group: "Kubernetes",
    });
  });
});
