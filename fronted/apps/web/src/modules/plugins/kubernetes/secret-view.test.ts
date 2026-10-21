import { describe, expect, it } from "vitest";
import {
  decodeDockerRegistryEntries,
  encodeBase64,
  encodeDockerRegistryData,
  encodeOpaqueData,
  formStateToSecret,
  secretToFormState,
} from "./secret-view";

describe("secret view", () => {
  it("round-trips docker registry secret data", () => {
    const encoded = encodeDockerRegistryData([
      { registry: "dockerhub.kubekey.local:443", username: "admin", password: "secret" },
    ]);
    const entries = decodeDockerRegistryEntries(encoded);
    expect(entries).toEqual([
      { registry: "dockerhub.kubekey.local:443", username: "admin", password: "secret" },
    ]);
  });

  it("encodes opaque secret entries", () => {
    expect(encodeOpaqueData([{ key: "config", value: "value" }])).toEqual({
      config: encodeBase64("value"),
    });
  });

  it("builds secret object from form state", () => {
    const original = {
      apiVersion: "v1",
      kind: "Secret",
      metadata: { name: "harbor-secret", namespace: "default" },
      type: "kubernetes.io/dockerconfigjson",
      data: {},
    };
    const form = secretToFormState(original);
    form.dockerEntries = [{ registry: "reg.local", username: "u", password: "p" }];
    const next = formStateToSecret(original, form);
    expect(next.metadata?.name).toBe("harbor-secret");
    expect(next.data?.[".dockerconfigjson"]).toBeTruthy();
  });
});
