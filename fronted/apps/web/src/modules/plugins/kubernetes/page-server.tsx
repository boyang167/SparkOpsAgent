import { KubernetesConsole, type KubernetesConsoleMode } from "./KubernetesConsole";
import { loadGovernanceContext } from "@/server/loaders/governance-context";

type RenderKubernetesPageOptions = {
  mode: KubernetesConsoleMode;
  title: string;
  description?: string;
  menu?: string;
};

export async function renderKubernetesPage(options: RenderKubernetesPageOptions) {
  const { canManage } = await loadGovernanceContext();
  return <KubernetesConsole {...options} canManage={canManage} />;
}
