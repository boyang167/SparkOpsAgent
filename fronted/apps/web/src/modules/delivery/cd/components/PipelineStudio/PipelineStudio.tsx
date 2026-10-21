"use client";

import Link from "next/link";
import { useCallback, useEffect, useMemo, useState } from "react";
import { InlineAlert } from "@/components/ui/InlineAlert";
import { autoLayoutStages } from "../../lib/dag-layout";
import {
  connectStages,
  createDroppedStage,
  disconnectStages,
  layoutWithNewStage,
  reconcileLayout,
} from "../../lib/flow-adapter";
import { mergePipelineFromYaml, pipelineToYaml, validatePipelineDefinition } from "../../lib/pipeline-yaml";
import { getMockPipeline } from "../../mock/data";
import type { PipelineLayout, PipelineStage, PipelineVM } from "../../types";
import { YamlEditorPanel } from "../shared/YamlEditorPanel";
import { PipelineCanvas } from "./PipelineCanvas";
import { StepPalette } from "./StepPalette";
import { StepPropertyPanel } from "./StepPropertyPanel";

interface PipelineStudioProps {
  pipelineId: string;
}

type EditorTab = "visual" | "yaml";

export function PipelineStudio({ pipelineId }: PipelineStudioProps) {
  const initial = useMemo(() => getMockPipeline(pipelineId), [pipelineId]);
  const [pipeline, setPipeline] = useState<PipelineVM | null>(initial ?? null);
  const [selectedStageId, setSelectedStageId] = useState<string | null>(
    initial?.definition.spec.stages[0]?.id ?? null,
  );
  const [tab, setTab] = useState<EditorTab>("visual");
  const [yamlText, setYamlText] = useState(initial ? pipelineToYaml(initial.definition) : "");
  const [yamlError, setYamlError] = useState<string | null>(null);
  const [validationErrors, setValidationErrors] = useState<string[]>([]);
  const [savedHint, setSavedHint] = useState<string | null>(null);

  const stages = pipeline?.definition.spec.stages ?? [];
  const selectedStage = stages.find((s) => s.id === selectedStageId) ?? null;

  useEffect(() => {
    if (!pipeline) return;
    if (tab === "yaml") {
      setYamlText(pipelineToYaml(pipeline.definition));
      setYamlError(null);
    }
  }, [pipeline, tab]);

  const commitStages = useCallback(
    (nextStages: PipelineStage[], nextLayout?: PipelineLayout) => {
      if (!pipeline) return false;
      const nextDefinition = {
        ...pipeline.definition,
        spec: { ...pipeline.definition.spec, stages: nextStages },
      };
      const errors = validatePipelineDefinition(nextDefinition);
      setValidationErrors(errors);
      if (errors.length > 0) return false;
      setPipeline({
        ...pipeline,
        definition: nextDefinition,
        layout: nextLayout ?? pipeline.layout,
      });
      setSavedHint(null);
      return true;
    },
    [pipeline],
  );

  const syncFromYaml = useCallback(() => {
    if (!pipeline) return;
    const result = mergePipelineFromYaml(pipeline, yamlText);
    setYamlError(result.errors[0] ?? null);
    setValidationErrors(result.errors);
    if (result.errors.length === 0) {
      setPipeline({
        ...result.pipeline,
        layout: reconcileLayout(pipeline.layout, result.pipeline.definition.spec.stages),
      });
    }
  }, [pipeline, yamlText]);

  function addStage(uses: string, position?: { x: number; y: number }) {
    if (!pipeline) return;
    const { stage } = createDroppedStage(stages, uses, position ?? { x: 48, y: 48 });
    const nextStages = [...stages, stage];
    const nextLayout = layoutWithNewStage(pipeline.layout, nextStages, stage.id, position);
    if (commitStages(nextStages, nextLayout)) {
      setSelectedStageId(stage.id);
    }
  }

  function updateSelectedStage(stage: PipelineStage) {
    if (!selectedStageId) return;
    const nextStages = stages.map((item) => (item.id === selectedStageId ? stage : item));
    if (stage.id !== selectedStageId) {
      const relinked = nextStages.map((item) => ({
        ...item,
        needs: item.needs?.map((need) => (need === selectedStageId ? stage.id : need)),
      }));
      const nextLayout: PipelineLayout = {
        stages: Object.fromEntries(
          Object.entries(pipeline?.layout.stages ?? {}).map(([key, value]) => [
            key === selectedStageId ? stage.id : key,
            value,
          ]),
        ),
      };
      if (commitStages(relinked, nextLayout)) {
        setSelectedStageId(stage.id);
      }
      return;
    }
    commitStages(nextStages);
  }

  function deleteStages(ids: string[]) {
    if (!pipeline || ids.length === 0) return;
    const idSet = new Set(ids);
    const nextStages = stages
      .filter((s) => !idSet.has(s.id))
      .map((s) => ({
        ...s,
        needs: s.needs?.filter((need) => !idSet.has(need)),
      }));
    const nextLayout = { stages: { ...pipeline.layout.stages } };
    for (const id of ids) {
      delete nextLayout.stages[id];
    }
    if (commitStages(nextStages, nextLayout)) {
      setSelectedStageId((current) => (current && idSet.has(current) ? (nextStages[0]?.id ?? null) : current));
    }
  }

  function handleConnect(sourceId: string, targetId: string) {
    const result = connectStages(stages, sourceId, targetId);
    setValidationErrors(result.errors);
    if (result.stages) {
      commitStages(result.stages);
    }
  }

  function handleDisconnect(sourceId: string, targetId: string) {
    commitStages(disconnectStages(stages, sourceId, targetId));
  }

  function onSaveMock() {
    setSavedHint(`已保存草稿（Mock）· v${(pipeline?.version ?? 0) + 1} · ${new Date().toLocaleTimeString()}`);
    setPipeline((current) => (current ? { ...current, version: current.version + 1 } : current));
  }

  if (!pipeline) {
    return (
      <InlineAlert tone="error">
        未找到 Pipeline。{" "}
        <Link href="/delivery/pipelines" className="link-inline">
          返回列表
        </Link>
      </InlineAlert>
    );
  }

  return (
    <div className="cd-studio stack-lg">
      <div className="cd-studio-toolbar">
        <div>
          <h2 className="cd-studio-title">{pipeline.name}</h2>
          <p className="muted text-sm">
            v{pipeline.version} · {pipeline.status} · 拖拽编排发布 DAG
          </p>
        </div>
        <div className="cd-studio-actions">
          <div className="cd-tab-switch">
            <button
              type="button"
              className={`cd-tab${tab === "visual" ? " is-active" : ""}`}
              onClick={() => setTab("visual")}
            >
              可视化
            </button>
            <button
              type="button"
              className={`cd-tab${tab === "yaml" ? " is-active" : ""}`}
              onClick={() => {
                setYamlText(pipelineToYaml(pipeline.definition));
                setTab("yaml");
              }}
            >
              YAML
            </button>
          </div>
          <button
            type="button"
            className="btn-secondary"
            onClick={() =>
              setPipeline((current) =>
                current ? { ...current, layout: autoLayoutStages(current.definition.spec.stages) } : current,
              )
            }
          >
            自动布局
          </button>
          <button type="button" className="btn" onClick={onSaveMock}>
            保存草稿
          </button>
        </div>
      </div>

      {savedHint ? <InlineAlert tone="success">{savedHint}</InlineAlert> : null}
      {validationErrors.length > 0 ? (
        <InlineAlert tone="warning">{validationErrors.join("；")}</InlineAlert>
      ) : null}

      {tab === "visual" ? (
        <div className="cd-studio-grid">
          <StepPalette onAdd={(uses) => addStage(uses)} />
          <div className="cd-canvas-column">
            <PipelineCanvas
              stages={stages}
              layout={pipeline.layout}
              selectedStageId={selectedStageId}
              onSelectStage={setSelectedStageId}
              onLayoutChange={(layout) => setPipeline({ ...pipeline, layout })}
              onConnectStages={handleConnect}
              onDisconnectStages={handleDisconnect}
              onDropStep={(uses, position) => addStage(uses, position)}
              onDeleteStages={deleteStages}
            />
          </div>
          <StepPropertyPanel
            stage={selectedStage}
            allStageIds={stages.map((s) => s.id)}
            onChange={updateSelectedStage}
            onDelete={() => {
              if (selectedStageId) deleteStages([selectedStageId]);
            }}
          />
        </div>
      ) : (
        <div className="cd-yaml-workspace">
          <div className="cd-yaml-toolbar">
            <p className="muted text-sm">编辑 YAML 后点击「应用到画布」同步 DAG。</p>
            <button type="button" className="btn" onClick={syncFromYaml}>
              应用到画布
            </button>
          </div>
          <YamlEditorPanel value={yamlText} onChange={setYamlText} error={yamlError} />
        </div>
      )}
    </div>
  );
}
