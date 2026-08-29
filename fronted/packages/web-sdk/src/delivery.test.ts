import { describe, expect, it } from "vitest";
import { parseDeploymentEvent } from "./delivery";

describe("parseDeploymentEvent", () => {
  it("parses stream deltas and completion", () => {
    expect(parseDeploymentEvent('{"delta":"building\\n"}')).toEqual({ delta: "building\n" });
    expect(parseDeploymentEvent('{"done":"true"}')).toEqual({ done: true });
  });
});
