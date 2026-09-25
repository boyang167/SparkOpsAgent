import type { SprintBoardVM, SprintStage } from "@sparkops/contracts/product";

export function buildStageMigrations(
  oldStages: SprintStage[],
  newStages: SprintStage[],
  board: SprintBoardVM | null,
): Record<string, string> {
  const newIds = new Set(newStages.map((stage) => stage.id));
  const fallback = newStages[0]?.id;
  const migrations: Record<string, string> = {};
  if (!fallback) return migrations;

  for (const oldStage of oldStages) {
    if (newIds.has(oldStage.id)) continue;
    const count = board?.columns.find((column) => column.stage.id === oldStage.id)?.work_items.length ?? 0;
    if (count > 0) {
      migrations[oldStage.id] = fallback;
    }
  }
  return migrations;
}
