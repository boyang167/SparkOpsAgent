import { redirect } from "next/navigation";

export default function CreateProjectPage() {
  redirect("/projects?create=1");
}
