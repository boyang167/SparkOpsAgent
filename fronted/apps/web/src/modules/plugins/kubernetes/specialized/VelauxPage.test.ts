import { describe, expect, it } from "vitest";
import { buildVelauxFrameProps, VELAUX_IFRAME_SANDBOX } from "../velaux-frame";

describe("Velaux page frame security", () => {
  it("keeps Velaux inside a sandboxed iframe only", () => {
    const props = buildVelauxFrameProps("prod/cluster");

    expect(props).toEqual({
      src: "/api/bff/v1/plugins/kubernetes/clusters/prod%2Fcluster/velaux/",
      sandbox: VELAUX_IFRAME_SANDBOX,
      referrerPolicy: "no-referrer",
    });
    expect(VELAUX_IFRAME_SANDBOX.split(" ")).not.toContain("allow-same-origin");
    expect(Object.keys(props)).not.toEqual(expect.arrayContaining(["href", "target"]));
  });
});
