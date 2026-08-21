import { redirect } from "next/navigation";

export default function LegacyRbacRedirect() {
  redirect("/admin/roles");
}
