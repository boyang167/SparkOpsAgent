import { describe, expect, it } from "vitest";
import { formatDeploymentLogText } from "./deployment-log-format";

describe("formatDeploymentLogText", () => {
  it("extracts message fields from deployment event lines", () => {
    const raw = [
      '{"event_id":1,"status":"queued","message":"deployment queued"}',
      '{"event_id":2,"status":"deploying","message":"deployment deploying"}',
      '{"event_id":3,"status":"succeeded","message":"deployment succeeded"}',
    ].join("\n");

    expect(formatDeploymentLogText(raw)).toBe(
      "deployment queued\ndeployment deploying\ndeployment succeeded",
    );
  });

  it("returns empty string for blank input", () => {
    expect(formatDeploymentLogText("")).toBe("");
    expect(formatDeploymentLogText("\n  \n")).toBe("");
  });
});
