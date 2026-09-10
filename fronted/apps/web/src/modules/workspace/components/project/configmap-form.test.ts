import { describe, expect, it } from "vitest";
import {
  dataToRows,
  isValidConfigMapKey,
  parseConfigMapYaml,
  rowsToData,
  sanitizeConfigMapKey,
  serializeConfigMapYaml,
} from "./configmap-form";

describe("configmap-form", () => {
  it("sanitizes uploaded filenames as keys", () => {
    expect(sanitizeConfigMapKey("app.conf")).toBe("app.conf");
    expect(sanitizeConfigMapKey("C:\\\\tmp\\\\nginx.conf")).toBe("nginx.conf");
    expect(sanitizeConfigMapKey("my file.yaml")).toBe("my-file.yaml");
  });

  it("ignores empty rows and rejects duplicate keys", () => {
    expect(
      rowsToData([
        { id: "1", key: "FOO", value: "bar" },
        { id: "2", key: "", value: "" },
      ]).data,
    ).toEqual({ FOO: "bar" });
    expect(rowsToData([{ id: "1", key: "A", value: "1" }, { id: "2", key: "A", value: "2" }]).error).toMatch(/重复/);
  });

  it("round-trips yaml data", () => {
    const yaml = serializeConfigMapYaml("test-repo", "test-workspace", { FOO: "bar", PATH: "/data" });
    const parsed = parseConfigMapYaml(yaml);
    expect(parsed.name).toBe("test-repo");
    expect(parsed.data).toEqual({ FOO: "bar", PATH: "/data" });
    expect(dataToRows(parsed.data).map((row) => row.key).sort()).toEqual(["FOO", "PATH"]);
  });

  it("parses flat yaml objects as data", () => {
    const parsed = parseConfigMapYaml("LOG_LEVEL: info\nPORT: 8080\n");
    expect(parsed.data).toEqual({ LOG_LEVEL: "info", PORT: "8080" });
    expect(isValidConfigMapKey("LOG_LEVEL")).toBe(true);
  });
});
