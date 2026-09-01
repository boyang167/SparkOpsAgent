import { describe, expect, it } from "vitest";
import {
  eventTimestamp,
  formatContainerCommand,
  formatContainerPorts,
  formatEnvironment,
  sortPodEvents,
  volumeSourceView,
} from "./pod-detail-view";

describe("pod-detail-view", () => {
  describe("eventTimestamp", () => {
    it("prefers lastTimestamp over eventTime and metadata.creationTimestamp", () => {
      const event = {
        lastTimestamp: "2026-09-01T12:00:00Z",
        eventTime: "2026-09-01T11:00:00Z",
        metadata: { creationTimestamp: "2026-09-01T10:00:00Z" },
      };
      expect(eventTimestamp(event)).toBe("2026-09-01T12:00:00Z");
    });

    it("falls back to eventTime when lastTimestamp is missing", () => {
      const event = {
        eventTime: "2026-09-01T11:00:00Z",
        metadata: { creationTimestamp: "2026-09-01T10:00:00Z" },
      };
      expect(eventTimestamp(event)).toBe("2026-09-01T11:00:00Z");
    });

    it("falls back to metadata.creationTimestamp when other timestamps are missing", () => {
      const event = {
        metadata: { creationTimestamp: "2026-09-01T10:00:00Z" },
      };
      expect(eventTimestamp(event)).toBe("2026-09-01T10:00:00Z");
    });

    it("returns undefined when no timestamp fields exist", () => {
      expect(eventTimestamp({})).toBeUndefined();
      expect(eventTimestamp({ metadata: {} })).toBeUndefined();
    });
  });

  describe("sortPodEvents", () => {
    it("sorts events in descending timestamp order", () => {
      const events = [
        { reason: "old", metadata: { creationTimestamp: "2026-09-01T08:00:00Z" } },
        { reason: "newest", lastTimestamp: "2026-09-01T12:00:00Z" },
        { reason: "middle", eventTime: "2026-09-01T10:00:00Z" },
      ];
      expect(sortPodEvents(events).map((item) => item.reason)).toEqual(["newest", "middle", "old"]);
    });

    it("uses lastTimestamp priority when sorting mixed events", () => {
      const events = [
        {
          reason: "with-eventTime-only",
          eventTime: "2026-09-01T11:00:00Z",
        },
        {
          reason: "with-lastTimestamp",
          lastTimestamp: "2026-09-01T12:00:00Z",
          eventTime: "2026-09-01T09:00:00Z",
        },
      ];
      expect(sortPodEvents(events).map((item) => item.reason)).toEqual([
        "with-lastTimestamp",
        "with-eventTime-only",
      ]);
    });

    it("places events without timestamps at the end", () => {
      const events = [
        { reason: "no-time" },
        { reason: "recent", lastTimestamp: "2026-09-01T12:00:00Z" },
      ];
      expect(sortPodEvents(events).map((item) => item.reason)).toEqual(["recent", "no-time"]);
    });

    it("does not mutate the original array", () => {
      const events = [
        { lastTimestamp: "2026-09-01T10:00:00Z" },
        { lastTimestamp: "2026-09-01T12:00:00Z" },
      ];
      const copy = [...events];
      sortPodEvents(events);
      expect(events).toEqual(copy);
    });
  });

  describe("formatContainerPorts", () => {
    it("formats ports with protocol and containerPort", () => {
      const container = {
        ports: [
          { containerPort: 8080, protocol: "TCP" },
          { containerPort: 53, protocol: "UDP" },
        ],
      };
      expect(formatContainerPorts(container)).toBe("TCP: 8080, UDP: 53");
    });

    it("defaults protocol to TCP when missing", () => {
      const container = {
        ports: [{ containerPort: 3000 }],
      };
      expect(formatContainerPorts(container)).toBe("TCP: 3000");
    });

    it("returns dash when no ports are defined", () => {
      expect(formatContainerPorts({})).toBe("-");
      expect(formatContainerPorts({ ports: [] })).toBe("-");
    });
  });

  describe("formatContainerCommand", () => {
    it("joins command and args with spaces", () => {
      const container = {
        command: ["/bin/sh", "-c"],
        args: ["echo", "hello"],
      };
      expect(formatContainerCommand(container)).toBe("/bin/sh -c echo hello");
    });

    it("returns command only when args are missing", () => {
      expect(formatContainerCommand({ command: ["nginx", "-g", "daemon off;"] })).toBe(
        "nginx -g daemon off;",
      );
    });

    it("returns args only when command is missing", () => {
      expect(formatContainerCommand({ args: ["--help"] })).toBe("--help");
    });

    it("returns dash when command and args are empty", () => {
      expect(formatContainerCommand({})).toBe("-");
    });
  });

  describe("formatEnvironment", () => {
    it("formats direct env values", () => {
      const container = {
        env: [{ name: "APP_ENV", value: "prod" }],
      };
      expect(formatEnvironment(container)).toEqual(["APP_ENV=prod"]);
    });

    it("formats valueFrom fieldRef", () => {
      const container = {
        env: [
          {
            name: "POD_NAME",
            valueFrom: { fieldRef: { fieldPath: "metadata.name" } },
          },
        ],
      };
      expect(formatEnvironment(container)).toEqual(["POD_NAME=<fieldRef metadata.name>"]);
    });

    it("formats valueFrom configMapKeyRef and secretKeyRef", () => {
      const container = {
        env: [
          {
            name: "CONFIG_KEY",
            valueFrom: { configMapKeyRef: { name: "app-config", key: "config.yaml" } },
          },
          {
            name: "SECRET_KEY",
            valueFrom: { secretKeyRef: { name: "app-secret", key: "password" } },
          },
        ],
      };
      expect(formatEnvironment(container)).toEqual([
        "CONFIG_KEY=<configMapKeyRef app-config/config.yaml>",
        "SECRET_KEY=<secretKeyRef app-secret/password>",
      ]);
    });

    it("stringifies unknown valueFrom sources safely", () => {
      const container = {
        env: [
          {
            name: "CUSTOM",
            valueFrom: { resourceFieldRef: { resource: "limits.cpu", divisor: "1m" } },
          },
        ],
      };
      expect(formatEnvironment(container)).toEqual([
        'CUSTOM=<valueFrom: {"resourceFieldRef":{"resource":"limits.cpu","divisor":"1m"}}>',
      ]);
    });

    it("returns empty array when env is missing", () => {
      expect(formatEnvironment({})).toEqual([]);
    });

    it("does not emit HTML in env output", () => {
      const container = {
        env: [{ name: "HTML", value: "<script>alert(1)</script>" }],
      };
      const lines = formatEnvironment(container);
      expect(lines[0]).toBe("HTML=<script>alert(1)</script>");
      expect(lines[0]).not.toContain("&lt;");
    });
  });

  describe("volumeSourceView", () => {
    it("recognizes hostPath volumes", () => {
      expect(
        volumeSourceView({
          name: "host-vol",
          hostPath: { path: "/data", type: "Directory" },
        }),
      ).toEqual({
        type: "hostPath",
        detail: "path: /data, type: Directory",
      });
    });

    it("recognizes configMap volumes", () => {
      expect(
        volumeSourceView({
          name: "cm-vol",
          configMap: { name: "app-config", defaultMode: 420 },
        }),
      ).toEqual({
        type: "configMap",
        detail: "name: app-config, defaultMode: 420",
      });
    });

    it("recognizes secret volumes", () => {
      expect(
        volumeSourceView({
          name: "secret-vol",
          secret: { secretName: "app-secret" },
        }),
      ).toEqual({
        type: "secret",
        detail: "secretName: app-secret",
      });
    });

    it("recognizes persistentVolumeClaim volumes", () => {
      expect(
        volumeSourceView({
          name: "pvc-vol",
          persistentVolumeClaim: { claimName: "data-pvc" },
        }),
      ).toEqual({
        type: "persistentVolumeClaim",
        detail: "claimName: data-pvc",
      });
    });

    it("recognizes emptyDir volumes", () => {
      expect(
        volumeSourceView({
          name: "cache",
          emptyDir: { medium: "Memory", sizeLimit: "1Gi" },
        }),
      ).toEqual({
        type: "emptyDir",
        detail: "medium: Memory, sizeLimit: 1Gi",
      });
    });

    it("recognizes downwardAPI volumes", () => {
      expect(
        volumeSourceView({
          name: "podinfo",
          downwardAPI: { items: [{ path: "labels", fieldRef: { fieldPath: "metadata.labels" } }] },
        }),
      ).toEqual({
        type: "downwardAPI",
        detail: 'items: [{"path":"labels","fieldRef":{"fieldPath":"metadata.labels"}}]',
      });
    });

    it("recognizes projected volumes", () => {
      expect(
        volumeSourceView({
          name: "all-in-one",
          projected: {
            sources: [{ configMap: { name: "cfg" } }, { secret: { name: "sec" } }],
          },
        }),
      ).toEqual({
        type: "projected",
        detail: 'sources: [{"configMap":{"name":"cfg"}},{"secret":{"name":"sec"}}]',
      });
    });

    it("shows first unknown source key and JSON summary", () => {
      expect(
        volumeSourceView({
          name: "custom-vol",
          csi: { driver: "ebs.csi.aws.com", volumeHandle: "vol-123" },
        }),
      ).toEqual({
        type: "csi",
        detail: '{"driver":"ebs.csi.aws.com","volumeHandle":"vol-123"}',
      });
    });

    it("returns dash placeholders for empty volume objects", () => {
      expect(volumeSourceView({ name: "empty" })).toEqual({ type: "-", detail: "-" });
    });
  });
});
