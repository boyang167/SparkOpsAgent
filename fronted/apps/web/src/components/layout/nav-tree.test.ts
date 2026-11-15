import { describe, expect, it } from "vitest";
import { DEFAULT_ENTITLEMENTS } from "./entitlements";
import { buildNavSearchItems } from "./nav-search";
import {
  BUSINESS_NAV_TREE,
  FILES_PLANE_NAV_GROUP,
  KNOWLEDGE_NAV_ITEM,
  filterBusinessDirectNavItems,
} from "./nav-tree";

describe("business navigation", () => {
  it("uses the product delivery information architecture", () => {
    expect(BUSINESS_NAV_TREE.map((group) => group.label)).toEqual([
      "产品交付",
      "运行观测",
      "配置与集成",
    ]);
    expect(BUSINESS_NAV_TREE[0]?.items.map((item) => item.label)).toEqual([
      "产品",
      "项目",
      "发布计划",
      "工作流",
      "定时构建",
      "模板中心",
    ]);
    expect(BUSINESS_NAV_TREE[1]?.items.map((item) => item.label)).toEqual([
      "运行实例",
      "监控概览",
      "日志查询",
    ]);
    expect(BUSINESS_NAV_TREE[2]?.items.map((item) => item.label)).toEqual([
      "密钥管理",
      "外部集成",
      "MCP 集成",
    ]);
  });

  it("keeps knowledge as a direct entitled entry", () => {
    expect(KNOWLEDGE_NAV_ITEM).toEqual({
      href: "/knowledge",
      label: "知识库",
      entitlement: "knowledge",
    });
  });

  it("exposes filesystem as a three-plane nav group", () => {
    expect(FILES_PLANE_NAV_GROUP.items.map((item) => item.label)).toEqual([
      "数据面",
      "控制面",
      "治理面",
    ]);
  });

  it("hides files navigation when filesystem entitlement is disabled", () => {
    const search = buildNavSearchItems({
      entitlements: { ...DEFAULT_ENTITLEMENTS, filesystem: false },
    });
    expect(search.some((item) => item.href.startsWith("/files"))).toBe(false);
  });
});
