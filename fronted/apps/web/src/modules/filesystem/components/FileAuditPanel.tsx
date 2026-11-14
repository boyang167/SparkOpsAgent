"use client";

import { createElement } from "react";
import { AuditWorkspace } from "./AuditWorkspace";

export type FileAuditPanelProps = {
  workspaceId: string;
  permissions?: string[];
};

/** @deprecated 工作台入口请直接使用 AuditWorkspace。 */
export function FileAuditPanel({
  workspaceId,
  permissions = ["file:admin"],
}: FileAuditPanelProps): React.ReactElement {
  return createElement(AuditWorkspace, { workspaceId, permissions });
}
