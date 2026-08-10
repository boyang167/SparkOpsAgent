import { redirect } from "next/navigation";

export default function LegacyPlatformAuditRedirect() {
  redirect("/monitor/platform-audit");
}
