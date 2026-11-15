import { PageHeader } from "@/components/ui/PageHeader";
import { DataPlaneModuleNav } from "@/modules/filesystem/components/DataPlaneModuleNav";

export default function FilesDataLayout({
  children,
}: {
  children: React.ReactNode;
}) {
  return (
    <div className="stack-md min-w-0 filesystem-data-layout">
      <PageHeader title="DATA PLANE" />
      <DataPlaneModuleNav />
      {children}
    </div>
  );
}