import { renderFilesystemConsoleModule } from "../../_lib/console-module-page";

export default async function FilesGovernanceLifecyclePage() {
  return renderFilesystemConsoleModule("governance", { governanceSection: "lifecycle" });
}