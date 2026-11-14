import { redirect } from "next/navigation";

export default function FilesystemAdminPage() {
  redirect("/files?tab=governance");
}
