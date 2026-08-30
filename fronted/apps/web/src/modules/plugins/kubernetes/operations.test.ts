import { describe, expect, it } from "vitest";
import { cloudTTYProxyPath, podOperationPath, toCommandQuery } from "./operations";

describe("kubernetes operation helpers", () => {
  it("encodes pod operation targets and repeated command arguments", () => {
    expect(podOperationPath("cluster", "dev", "api-0", "logs")).toBe(
      "/api/bff/v1/plugins/kubernetes/clusters/cluster/namespaces/dev/pods/api-0/logs",
    );
    expect(toCommandQuery(["sh", "-c", "echo hello"])).toBe(
      "command=sh&command=-c&command=echo+hello",
    );
  });

  it("builds a sandboxed CloudTTY service proxy path from the CloudShell ref", () => {
    expect(cloudTTYProxyPath("prod", "default", "sparkops-cloudtty-abcd")).toBe(
      "/api/bff/v1/plugins/kubernetes/clusters/prod/proxy/api/v1/namespaces/default/services/http%3Asparkops-cloudtty-abcd%3A7681/proxy/",
    );
  });
});
