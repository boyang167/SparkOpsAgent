import { PageHeader } from "@/components/ui/PageHeader";
import { ProclaimsPanel } from "@/modules/platform_admin/components/ProclaimsPanel";

export default function ProclaimsPage() {
  return (
    <div className="stack-lg">
      <PageHeader eyebrow="Admin" title="公告管理" description="平台公告（对应旧 /proclaim）" />
      <ProclaimsPanel />
    </div>
  );
}
