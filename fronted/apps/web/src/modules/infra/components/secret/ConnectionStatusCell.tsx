import type { ConnectionStatus } from "./secret-utils";
import { CheckIcon, CloseIcon } from "./SecretIcons";
import { connectionBadgeClass } from "./secret-utils";

interface ConnectionStatusCellProps {
  status?: ConnectionStatus;
  bulkTesting?: boolean;
  isActive?: boolean;
}

export function ConnectionStatusCell({ status, bulkTesting, isActive }: ConnectionStatusCellProps) {
  if (status === "testing" || (bulkTesting && isActive && !status)) {
    return <span className="secret-conn-pending">测试中</span>;
  }
  if (status === "success") {
    return (
      <span className={connectionBadgeClass("success")}>
        <CheckIcon />
        成功
      </span>
    );
  }
  if (status === "failed") {
    return (
      <span className={connectionBadgeClass("failed")}>
        <CloseIcon />
        失败
      </span>
    );
  }
  return <span className="secret-conn-untested">未测试</span>;
}
