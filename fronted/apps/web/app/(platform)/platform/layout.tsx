import { requirePlatformConsoleAccess } from "@/server/loaders/console-access";

export default async function PlatformConsoleLayout({ children }: { children: React.ReactNode }) {
  await requirePlatformConsoleAccess();
  return children;
}
