import { describe, expect, it } from "vitest";
import {
  COMMAND_KIND_LABELS,
  buildStaticCommandItems,
  groupCommandResults,
} from "./command-search-index";

describe("filesystem command search labels", () => {
  it("uses 文件中心 for the entry and filesystem category", () => {
    const items = buildStaticCommandItems({
      canManageAdmin: false,
      isSuperAdmin: false,
    });
    const fileItem = items.find((item) => item.href === "/files");

    expect(fileItem?.label).toBe("文件中心");
    expect(groupCommandResults(fileItem ? [fileItem] : [])[0]?.label).toBe("文件中心");
    expect(COMMAND_KIND_LABELS.filesystem).toBe("文件中心");
  });
});
