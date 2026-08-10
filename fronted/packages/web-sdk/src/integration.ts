import type { ModelVM } from "@sparkops/contracts/integration";
import { bffFetch } from "./client";

export async function listModels() {
  const data = await bffFetch<{ items: ModelVM[] }>("/models");
  return data.items;
}
