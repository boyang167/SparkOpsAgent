import { redirect } from "next/navigation";
import { loadSession } from "@/server/loaders/session";

export default async function HomePage() {
  const session = await loadSession();
  redirect(session ? "/dashboard" : "/login");
}
