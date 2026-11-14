// @vitest-environment jsdom

import "@testing-library/jest-dom/vitest";
import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { createElement } from "react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { Drawer } from "./Drawer";

describe("Drawer", () => {
  afterEach(() => cleanup());

  it("focuses inside, traps focus and restores the trigger on close", async () => {
    const trigger = document.createElement("button");
    trigger.textContent = "打开";
    document.body.appendChild(trigger);
    trigger.focus();
    const onClose = vi.fn();
    const rendered = render(
      createElement(
        Drawer,
        {
          open: true,
          onClose,
          title: "详情",
          children: createElement("button", { type: "button" }, "最后操作"),
        },
      ),
    );

    const close = await screen.findByRole("button", { name: "关闭" });
    await waitFor(() => expect(close).toHaveFocus());
    const last = screen.getByRole("button", { name: "最后操作" });
    last.focus();
    fireEvent.keyDown(document, { key: "Tab" });
    expect(close).toHaveFocus();
    fireEvent.keyDown(document, { key: "Tab", shiftKey: true });
    expect(last).toHaveFocus();

    rendered.rerender(
      createElement(
        Drawer,
        {
          open: false,
          onClose,
          title: "详情",
          children: createElement("button", { type: "button" }, "最后操作"),
        },
      ),
    );
    expect(trigger).toHaveFocus();
    trigger.remove();
  });
});
