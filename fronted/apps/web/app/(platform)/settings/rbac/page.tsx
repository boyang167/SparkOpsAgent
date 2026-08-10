import { redirect } from "next/navigation";

export default function LegacyRbacRedirect() {
  redirect("/permission/roles");
}
