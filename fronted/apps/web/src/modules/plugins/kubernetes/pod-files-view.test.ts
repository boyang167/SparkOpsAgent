import { describe, expect, it } from "vitest";
import { joinPodFilePath, parseLsOutput, podFileBreadcrumbs, podFilePathFromBreadcrumbs } from "./pod-files-view";

describe("pod files view", () => {
  it("parses directory, file and symlink rows from ls -la output", () => {
    const output = [
      "total 0",
      "drwxr-xr-x    2 root     root          4096 Sep  2 19:00 dev",
      "-rw-r--r--    1 root     root            12 Sep  2 19:00 hello.txt",
      "lrwxrwxrwx    1 root     root             7 Sep  2 19:00 stderr -> /proc/self/fd/2",
    ].join("\n");

    expect(parseLsOutput(output)).toEqual([
      {
        name: "dev",
        size: "4096",
        mode: "drwxr-xr-x",
        user: "root",
        group: "root",
        modTime: "Sep 2 19:00",
        isDir: true,
        isSymlink: false,
        link: "",
      },
      {
        name: "hello.txt",
        size: "12",
        mode: "-rw-r--r--",
        user: "root",
        group: "root",
        modTime: "Sep 2 19:00",
        isDir: false,
        isSymlink: false,
        link: "",
      },
      {
        name: "stderr",
        size: "7",
        mode: "lrwxrwxrwx",
        user: "root",
        group: "root",
        modTime: "Sep 2 19:00",
        isDir: false,
        isSymlink: true,
        link: "/proc/self/fd/2",
      },
    ]);
  });

  it("parses device entries and skips dot directories", () => {
    const output = "crw-rw-rw-    1 root     root        1,   7 Sep  2 19:00 full\ndrwxr-xr-x    1 root     root          4096 Sep  2 19:00 .";
    const [entry] = parseLsOutput(output);
    expect(entry?.size).toBe("1, 7");
    expect(entry?.name).toBe("full");
  });

  it("builds pod file paths and breadcrumbs", () => {
    expect(joinPodFilePath("/", "dev")).toBe("/dev");
    expect(joinPodFilePath("/dev", "shm")).toBe("/dev/shm");
    expect(podFileBreadcrumbs("/dev/shm")).toEqual(["dev", "shm"]);
    expect(podFilePathFromBreadcrumbs(["dev", "shm"])).toBe("/dev/shm");
  });
});
