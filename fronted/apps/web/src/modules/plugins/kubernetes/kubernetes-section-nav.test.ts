import { describe, expect, it } from "vitest";
import { sectionForPath, visibleSectionForPath } from "./kubernetes-section-nav";

describe("KubernetesSectionNav", () => {
  it.each([
    ["/plugins/kubernetes/gateway", "/plugins/kubernetes/networks"],
    ["/plugins/kubernetes/storages", "/plugins/kubernetes/configs"],
    ["/plugins/kubernetes/records", "/plugins/kubernetes/access-controls"],
    ["/plugins/kubernetes/cloudtty", "/plugins/kubernetes/ops-tools"],
    ["/plugins/kubernetes/velaux", "/plugins/kubernetes/ops-tools"],
    ["/plugins/kubernetes/diagnosis", "/plugins/kubernetes/ops-tools"],
  ])("maps %s to consolidated section %s", (pathname, sectionHref) => {
    expect(sectionForPath(pathname)?.href).toBe(sectionHref);
  });

  it("does not add a secondary nav for single-page sections", () => {
    expect(sectionForPath("/plugins/kubernetes/clusters")?.items).toHaveLength(1);
  });

  it("only exposes secondary links present in the member menu tree", () => {
    const section = visibleSectionForPath(
      "/plugins/kubernetes/records",
      new Set(["/plugins/kubernetes/records"]),
    );
    expect(section?.items).toEqual([
      { href: "/plugins/kubernetes/records", label: "终端审计" },
    ]);
  });
});
