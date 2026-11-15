import { renderFilesystemConsoleModule } from "../../_lib/console-module-page";

export default async function FilesControlPoolsPage() {
  return renderFilesystemConsoleModule("governance", { governanceSection: "pools" });
}