// @vitest-environment jsdom

import "@testing-library/jest-dom/vitest";
import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { createElement, useState } from "react";
import { afterEach, describe, expect, it } from "vitest";
import { useModalFocus } from "./modal-focus";

function ModalHarness() {
  const [open, setOpen] = useState(false);
  const panelRef = useModalFocus<HTMLDivElement>(open);
  return createElement(
    "div",
    null,
    createElement(
      "button",
      { type: "button", onClick: () => setOpen(true) },
      "打开移动对话框",
    ),
    open
      ? createElement(
          "div",
          { ref: panelRef, role: "dialog", "aria-label": "移动到…" },
          createElement(
            "button",
            { type: "button", onClick: () => setOpen(false), "aria-label": "关闭" },
            "关闭",
          ),
          createElement("button", { type: "button" }, "目录选项"),
          createElement("button", { type: "button" }, "确认移动"),
        )
      : null,
  );
}

describe("Modal focus management", () => {
  afterEach(cleanup);

  it("moves focus inside, loops Tab, and restores the trigger on close", async () => {
    const user = userEvent.setup();
    render(createElement(ModalHarness));
    const trigger = screen.getByRole("button", { name: "打开移动对话框" });

    await user.click(trigger);

    const close = await screen.findByRole("button", { name: "关闭" });
    await waitFor(() => expect(close).toHaveFocus());
    const confirm = screen.getByRole("button", { name: "确认移动" });

    fireEvent.keyDown(close, { key: "Tab", shiftKey: true });
    expect(confirm).toHaveFocus();
    fireEvent.keyDown(confirm, { key: "Tab" });
    expect(close).toHaveFocus();

    await user.click(close);
    expect(trigger).toHaveFocus();
  });
});
