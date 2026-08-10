import { PageHeader } from "@/components/ui/PageHeader";
import { ToolsPanel } from "@/modules/platform_admin/components/ToolsPanel";

export default function ToolsPage() {
  return (
    <div className="stack-lg">
      <PageHeader eyebrow="Tools" title="工具箱" description="运维工具集合（对应旧 /tools）" />
      <ToolsPanel />
    </div>
  );
}
