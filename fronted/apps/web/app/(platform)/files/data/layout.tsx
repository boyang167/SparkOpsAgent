import { DataPlaneModuleNav } from "@/modules/filesystem/components/DataPlaneModuleNav";
import { DataPlanePageHeader } from "@/modules/filesystem/components/DataPlanePageHeader";

export default function FilesDataLayout({
  children,
}: {
  children: React.ReactNode;
}) {
  return (
    <div className="stack-md min-w-0 filesystem-data-layout">
      <DataPlanePageHeader />
      <DataPlaneModuleNav />
      {children}
    </div>
  );
}