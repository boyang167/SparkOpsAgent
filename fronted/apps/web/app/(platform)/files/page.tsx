import { redirect } from "next/navigation";
import {
  FILESYSTEM_DEFAULT_ENTRY,
  migrateLegacyFilesPath,
} from "@/modules/filesystem/components/filesystem-console-nav";

export default async function FilesRootPage({
  searchParams,
}: {
  searchParams: Promise<Record<string, string | string[] | undefined>>;
}) {
  const raw = await searchParams;
  const search = new URLSearchParams();
  for (const [key, value] of Object.entries(raw)) {
    const first = Array.isArray(value) ? value[0] : value;
    if (first != null && first !== "") search.set(key, first);
  }
  const migrated = migrateLegacyFilesPath("/files", search);
  redirect(migrated ?? FILESYSTEM_DEFAULT_ENTRY);
}