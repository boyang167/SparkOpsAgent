import { describe, expect, it } from "vitest";
import { RegisterWorkspaceClusterCommandSchema } from "./index";

describe("RegisterWorkspaceClusterCommandSchema", () => {
  it("keeps kubeconfig credential fields when registering a workspace cluster", () => {
    const parsed = RegisterWorkspaceClusterCommandSchema.parse({
      cluster_name: "dev",
      server: "https://192.168.5.21:6443",
      api_address: "https://192.168.5.21:6443",
      kube_type: 1,
      kube_config: "apiVersion: v1\nkind: Config\n",
      bearer_token: "",
      user_name: "admin",
    });

    expect(parsed).toMatchObject({
      cluster_name: "dev",
      server: "https://192.168.5.21:6443",
      api_address: "https://192.168.5.21:6443",
      kube_type: 1,
      kube_config: "apiVersion: v1\nkind: Config\n",
      user_name: "admin",
    });
  });
});
