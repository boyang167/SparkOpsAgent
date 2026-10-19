import { describe, expect, it } from "vitest";
import { unreadBadgeText } from "./notification-events";
import {
  groupNotificationsByDay,
  notificationDayLabel,
  notificationKind,
  notificationTimeLabel,
  parseNotificationBody,
} from "./notification-view";

const now = new Date(2026, 9, 13, 19, 40, 0);

describe("parseNotificationBody", () => {
  it("turns status bodies into a localized pill", () => {
    expect(parseNotificationBody("状态：success")).toEqual({ status: { label: "成功", tone: "success" }, text: "" });
    expect(parseNotificationBody("状态：failed").status?.tone).toBe("danger");
  });

  it("keeps free text bodies", () => {
    expect(parseNotificationBody("对话已完成")).toEqual({ status: null, text: "对话已完成" });
  });
});

describe("notification time helpers", () => {
  it("labels days relative to now", () => {
    expect(notificationDayLabel(new Date(2026, 9, 13, 8).toISOString(), now)).toBe("今天");
    expect(notificationDayLabel(new Date(2026, 9, 12, 23).toISOString(), now)).toBe("昨天");
    expect(notificationDayLabel(new Date(2026, 9, 1).toISOString(), now)).toBe("10月1日");
  });

  it("formats relative times", () => {
    expect(notificationTimeLabel(new Date(2026, 9, 13, 19, 39, 30).toISOString(), now)).toBe("刚刚");
    expect(notificationTimeLabel(new Date(2026, 9, 13, 19, 10).toISOString(), now)).toBe("30 分钟前");
    expect(notificationTimeLabel(new Date(2026, 9, 11, 22, 27).toISOString(), now)).toBe("22:27");
  });

  it("groups consecutive items by day", () => {
    const groups = groupNotificationsByDay(
      [
        { created_at: new Date(2026, 9, 13, 9).toISOString() },
        { created_at: new Date(2026, 9, 13, 8).toISOString() },
        { created_at: new Date(2026, 9, 11, 22).toISOString() },
      ],
      now,
    );
    expect(groups.map((group) => [group.label, group.items.length])).toEqual([
      ["今天", 2],
      ["10月11日", 1],
    ]);
  });
});

describe("unreadBadgeText", () => {
  it("hides zero and caps large counts", () => {
    expect(unreadBadgeText(0)).toBe("");
    expect(unreadBadgeText(21)).toBe("21");
    expect(unreadBadgeText(120)).toBe("99+");
  });
});

describe("notificationKind", () => {
  it("maps backend types", () => {
    expect(notificationKind("build.completed")).toBe("build");
    expect(notificationKind("workflow.run.completed")).toBe("workflow");
    expect(notificationKind("run.completed")).toBe("agent");
    expect(notificationKind("project.review.pending")).toBe("project");
  });
});
