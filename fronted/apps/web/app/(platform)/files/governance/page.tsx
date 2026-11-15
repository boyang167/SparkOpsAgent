import { redirect } from "next/navigation";
import { filesystemPlaneHref } from "@/modules/filesystem/components/filesystem-console-nav";

export default function FilesGovernancePlanePage() {
  redirect(filesystemPlaneHref("governance"));
}