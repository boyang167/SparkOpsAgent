import { redirect } from "next/navigation";

export default function LegacyStatisticsRedirect() {
  redirect("/monitor/statistics");
}
