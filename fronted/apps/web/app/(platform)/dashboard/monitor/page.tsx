import { redirect } from "next/navigation";

export default function LegacyMonitorRedirect() {
  redirect("/monitor/overview");
}
