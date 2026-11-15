import { redirect } from "next/navigation";
import { FILESYSTEM_DEFAULT_ENTRY } from "@/modules/filesystem/components/filesystem-console-nav";

export default function FilesDataPlanePage() {
  redirect(FILESYSTEM_DEFAULT_ENTRY);
}