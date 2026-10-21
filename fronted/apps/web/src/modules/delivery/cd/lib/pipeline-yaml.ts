import { stringify } from "yaml";
import {
  assertSafeYamlObject,
  parseYamlEditorDocument,
  stringifyYamlEditorDocument,
} from "@/modules/plugins/kubernetes/yaml-document";
import { getStepCatalogEntry } from "../mock/data";
import type { PipelineDefinition, PipelineLayout, PipelineStage, PipelineVM } from "../types";

export interface PipelineYamlDocument {
  apiVersion: string;
  kind: string;
  metadata: Record<string, unknown>;
  spec: Record<string, unknown>;
}

export function pipelineToYaml(definition: PipelineDefinition): string {
  const doc: PipelineYamlDocument = {
    apiVersion: definition.apiVersion,
    kind: definition.kind,
    metadata: {
      name: definition.metadata.name,
      ...(definition.metadata.description ? { description: definition.metadata.description } : {}),
    },
    spec: {
      parameters: definition.spec.parameters,
      stages: definition.spec.stages.map((stage) => sanitizeStage(stage)),
    },
  };
  return stringifyYamlEditorDocument(doc);
}

export function yamlToPipeline(text: string): PipelineDefinition {
  const doc = parseYamlEditorDocument(text) as unknown as PipelineYamlDocument;
  if (doc.apiVersion !== "cd.sparkops.io/v1" || doc.kind !== "Pipeline") {
    throw new Error("YAML 必须是 cd.sparkops.io/v1 Pipeline 文档");
  }
  const metadata = doc.metadata ?? {};
  const spec = doc.spec ?? {};
  const stages = Array.isArray(spec.stages) ? spec.stages : [];
  const parameters = Array.isArray(spec.parameters) ? spec.parameters : [];

  const definition: PipelineDefinition = {
    apiVersion: "cd.sparkops.io/v1",
    kind: "Pipeline",
    metadata: {
      name: String(metadata.name ?? "unnamed"),
      description: metadata.description ? String(metadata.description) : undefined,
    },
    spec: {
      parameters: parameters as PipelineDefinition["spec"]["parameters"],
      stages: stages as PipelineStage[],
    },
  };
  validatePipelineDefinition(definition);
  return definition;
}

export function validatePipelineDefinition(definition: PipelineDefinition): string[] {
  const errors: string[] = [];
  const ids = new Set<string>();

  for (const stage of definition.spec.stages) {
    if (!stage.id?.trim()) {
      errors.push("存在缺少 id 的 stage");
      continue;
    }
    if (ids.has(stage.id)) {
      errors.push(`stage id 重复: ${stage.id}`);
    }
    ids.add(stage.id);
    if (!stage.steps?.length) {
      errors.push(`stage「${stage.id}」至少需要一个 step`);
    }
    for (const step of stage.steps ?? []) {
      if (step.uses && !getStepCatalogEntry(step.uses)) {
        errors.push(`stage「${stage.id}」的 uses「${step.uses}」不在 Step Catalog 中`);
      }
    }
  }

  for (const stage of definition.spec.stages) {
    for (const need of stage.needs ?? []) {
      if (!ids.has(need)) {
        errors.push(`stage「${stage.id}」依赖不存在的节点: ${need}`);
      }
    }
  }

  if (hasCycle(definition.spec.stages)) {
    errors.push("Pipeline DAG 存在环，请检查 needs 依赖");
  }

  return errors;
}

function hasCycle(stages: PipelineStage[]): boolean {
  const graph = new Map<string, string[]>();
  for (const stage of stages) {
    graph.set(stage.id, stage.needs ?? []);
  }
  const visiting = new Set<string>();
  const visited = new Set<string>();

  function dfs(id: string): boolean {
    if (visiting.has(id)) return true;
    if (visited.has(id)) return false;
    visiting.add(id);
    for (const next of graph.get(id) ?? []) {
      if (dfs(next)) return true;
    }
    visiting.delete(id);
    visited.add(id);
    return false;
  }

  for (const id of graph.keys()) {
    if (dfs(id)) return true;
  }
  return false;
}

function sanitizeStage(stage: PipelineStage): Record<string, unknown> {
  const out: Record<string, unknown> = {
    id: stage.id,
    steps: stage.steps.map((step) => {
      const item: Record<string, unknown> = { uses: step.uses };
      if (step.with && Object.keys(step.with).length > 0) item.with = step.with;
      if (step.timeout) item.timeout = step.timeout;
      if (step.retry) item.retry = step.retry;
      return item;
    }),
  };
  if (stage.needs?.length) out.needs = stage.needs;
  if (stage.if) out.if = stage.if;
  return out;
}

export function mergePipelineFromYaml(
  pipeline: PipelineVM,
  yamlText: string,
): { pipeline: PipelineVM; errors: string[] } {
  try {
    const definition = yamlToPipeline(yamlText);
    const errors = validatePipelineDefinition(definition);
    if (errors.length > 0) {
      return { pipeline, errors };
    }
    return {
      pipeline: {
        ...pipeline,
        definition,
        name: definition.metadata.name,
        description: definition.metadata.description ?? pipeline.description,
      },
      errors,
    };
  } catch (err) {
    return {
      pipeline,
      errors: [err instanceof Error ? err.message : "YAML 解析失败"],
    };
  }
}

export function layoutToYaml(layout: PipelineLayout): string {
  assertSafeYamlObject(layout);
  return stringify(layout);
}

export function parseLayoutYaml(text: string): PipelineLayout {
  const value = parseYamlEditorDocument(text);
  const stages = (value as { stages?: Record<string, { x: number; y: number }> }).stages ?? {};
  return { stages };
}
