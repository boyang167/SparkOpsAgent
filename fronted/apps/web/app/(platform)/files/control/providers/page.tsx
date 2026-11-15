import { renderFilesystemConsoleModule } from "../../_lib/console-module-page";

export default async function FilesControlProvidersPage() {
  return renderFilesystemConsoleModule("governance", { governanceSection: "providers" });
}