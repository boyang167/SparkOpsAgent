import { describe, expect, it } from "vitest";
import { approvalJobSpecToRecord, parseApprovalJobSpec } from "./approval-job-spec";

describe("approval job spec", () => {
  it("parses and serializes approval configuration", () => {
    const spec = parseApprovalJobSpec({
      provider: "sparkops",
      approver_ids: ["m1", "m2"],
      required_approvals: 2,
      timeout_minutes: 90,
      notify: [],
    });

    expect(spec.approverIds).toEqual(["m1", "m2"]);
    expect(spec.requiredApprovals).toBe(2);
    expect(spec.timeoutMinutes).toBe(90);

    const record = approvalJobSpecToRecord(spec);
    expect(record).toMatchObject({
      provider: "sparkops",
      approver_ids: ["m1", "m2"],
      required_approvals: 2,
      timeout_minutes: 90,
    });
  });
});
