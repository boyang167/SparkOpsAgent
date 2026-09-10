import { describe, expect, it } from "vitest";
import { isValidWebhookURL, moveWebhookEvents, toggleWebhookEvent } from "./webhook-form";

describe("webhook-form", () => {
  it("accepts http urls", () => {
    expect(isValidWebhookURL("https://hooks.example.com/sparkops")).toBe(true);
    expect(isValidWebhookURL("ftp://x")).toBe(false);
  });

  it("moves events between lists", () => {
    expect(moveWebhookEvents(["Build"], ["Deploy"], true)).toEqual(["Build", "Deploy"]);
    expect(moveWebhookEvents(["Build", "Deploy"], ["Build"], false)).toEqual(["Deploy"]);
  });

  it("toggles a single event", () => {
    expect(toggleWebhookEvent(["Build"], "Deploy")).toEqual(["Build", "Deploy"]);
    expect(toggleWebhookEvent(["Build"], "Build")).toEqual([]);
  });
});
