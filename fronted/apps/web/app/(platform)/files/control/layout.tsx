import { PageHeader } from "@/components/ui/PageHeader";

export default function FilesControlLayout({
  children,
}: {
  children: React.ReactNode;
}) {
  return (
    <div className="stack-lg min-w-0">
      <PageHeader title="CONTROL PLANE" />
      {children}
    </div>
  );
}