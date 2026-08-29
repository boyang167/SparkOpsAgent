import type { DeploymentStatus } from "@sparkops/contracts/delivery";

const ACTIVE_DEPLOYMENT_STATUSES = new Set<DeploymentStatus>([
  "queued",
  "preparing",
  "building",
  "deploying",
]);

export function shouldContinueDeploymentPoll(
  status: DeploymentStatus,
  requestGeneration: number,
  currentGeneration: number,
) {
  return requestGeneration === currentGeneration && ACTIVE_DEPLOYMENT_STATUSES.has(status);
}

export function createSerialPoller<T, TimerToken>(options: {
  refresh: () => Promise<T>;
  onValue: (value: T) => void;
  onError: (error: unknown) => void;
  shouldContinue: (value: T) => boolean;
  schedule: (callback: () => void, delayMs: number) => TimerToken;
  cancel: (token: TimerToken) => void;
  delayMs: number;
}) {
  let generation = 0;
  let timer: TimerToken | null = null;

  async function poll(requestGeneration: number) {
    try {
      const value = await options.refresh();
      if (generation !== requestGeneration) return;
      options.onValue(value);
      if (!options.shouldContinue(value)) return;
    } catch (error) {
      if (generation !== requestGeneration) return;
      options.onError(error);
    }
    if (generation === requestGeneration) {
      timer = options.schedule(() => void poll(requestGeneration), options.delayMs);
    }
  }

  return {
    start() {
      generation += 1;
      void poll(generation);
    },
    stop() {
      generation += 1;
      if (timer !== null) {
        options.cancel(timer);
        timer = null;
      }
    },
  };
}
