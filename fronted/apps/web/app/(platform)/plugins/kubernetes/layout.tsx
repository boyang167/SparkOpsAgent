import { KubernetesSectionNav } from "@/modules/plugins/kubernetes/KubernetesSectionNav";

export default function KubernetesLayout({ children }: { children: React.ReactNode }) {
  return (
    <div className="stack-md">
      <KubernetesSectionNav />
      {children}
    </div>
  );
}
