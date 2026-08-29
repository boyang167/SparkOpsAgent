import { describe, expect, it } from "vitest";
import {
  createSerialPoller,
  shouldContinueDeploymentPoll,
} from "./deployment-poll";

describe("shouldContinueDeploymentPoll", () => {
  it("continues only for the current active run generation", () => {
    expect(shouldContinueDeploymentPoll("building", 2, 2)).toBe(true);
    expect(shouldContinueDeploymentPoll("building", 1, 2)).toBe(false);
    expect(shouldContinueDeploymentPoll("succeeded", 2, 2)).toBe(false);
  });

  it("schedules the next refresh only after the current refresh resolves", async () => {
    const resolvers: Array<(value: string) => void> = [];
    const scheduled: Array<() => void> = [];
    let calls = 0;
    const poller = createSerialPoller({
      refresh: () =>
        new Promise<string>((resolve) => {
          calls += 1;
          resolvers.push(resolve);
        }),
      onValue: () => undefined,
      onError: () => undefined,
      shouldContinue: () => true,
      schedule: (callback) => {
        scheduled.push(callback);
        return callback;
      },
      cancel: () => undefined,
      delayMs: 10,
    });

    poller.start();
    expect(calls).toBe(1);
    expect(scheduled).toHaveLength(0);
    resolvers[0]?.("first");
    await Promise.resolve();
    await Promise.resolve();
    expect(scheduled).toHaveLength(1);

    scheduled[0]?.();
    expect(calls).toBe(2);
    expect(scheduled).toHaveLength(1);
    poller.stop();
  });
});
