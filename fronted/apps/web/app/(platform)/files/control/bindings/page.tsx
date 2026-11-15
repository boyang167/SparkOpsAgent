import { renderFilesystemConsoleModule } from "../../_lib/console-module-page";

export default async function FilesControlBindingsPage() {
  return renderFilesystemConsoleModule("governance", { governanceSection: "bindings" });
}