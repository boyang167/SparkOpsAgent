import { PageHeader } from "@/components/ui/PageHeader";

export default function FilesGovernanceLayout({
  children,
}: {
  children: React.ReactNode;
}) {
  return (
    <div className="stack-lg min-w-0">
      <PageHeader title="GOVERNANCE PLANE" />
      {children}
    </div>
  );
}