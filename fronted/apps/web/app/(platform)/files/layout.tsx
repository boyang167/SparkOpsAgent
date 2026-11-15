import { loadFilesystemConsoleContext } from "./_lib/console-context";

export default async function FilesLayout({
  children,
}: {
  children: React.ReactNode;
}) {
  const ctx = await loadFilesystemConsoleContext();

  return (
    <div className="stack-lg min-w-0">
      <div className="filesystem-console-main min-w-0">
        {!ctx.ok ? ctx.element : children}
      </div>
    </div>
  );
}