"use client";

import { useCallback, useEffect, useState } from "react";
import { useRouter } from "next/navigation";
import { product } from "@sparkops/web-sdk";
import { BffError } from "@sparkops/contracts";
import type {
  EnvironmentVM,
  ProductProjectVM,
  WorkflowAdvancedVM,
  WorkflowNotificationVM,
  WorkflowTriggerVM,
  WorkflowVariableVM,
} from "@sparkops/contracts/product";
import { ProductWorkflowEditor } from "./ProductWorkflowEditor";
import { emptyStage } from "./workflow-editor-utils";

interface ProductWorkflowEditorPageProps {
  workspaceId: string;
  productId: string;
  workflowId: string;
}

export function ProductWorkflowEditorPage({
  workspaceId,
  productId,
  workflowId,
}: ProductWorkflowEditorPageProps) {
  const router = useRouter();
  const [name, setName] = useState("");
  const [workflowKey, setWorkflowKey] = useState("");
  const [description, setDescription] = useState("");
  const [status, setStatus] = useState("draft");
  const [stages, setStages] = useState<WorkflowStageVM[]>([emptyStage(1)]);
  const [notifications, setNotifications] = useState<WorkflowNotificationVM[]>([]);
  const [variables, setVariables] = useState<WorkflowVariableVM[]>([]);
  const [triggers, setTriggers] = useState<WorkflowTriggerVM[]>([]);
  const [advanced, setAdvanced] = useState<WorkflowAdvancedVM>({ concurrency: 1, timeout_minutes: 0 });
  const [projects, setProjects] = useState<ProductProjectVM[]>([]);
  const [environments, setEnvironments] = useState<EnvironmentVM[]>([]);
  const [loading, setLoading] = useState(true);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const load = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const [wf, projectItems, envItems] = await Promise.all([
        product.getProductWorkflow(workspaceId, productId, workflowId),
        product.listProductProjects(workspaceId, productId),
        product.listEnvironments(workspaceId, productId),
      ]);
      setName(wf.name);
      setWorkflowKey(wf.key);
      setDescription(wf.description ?? "");
      setStatus(wf.status);
      setStages(wf.stages?.length ? wf.stages : [emptyStage(1)]);
      setNotifications(wf.notifications ?? []);
      setVariables(wf.variables ?? []);
      setTriggers(wf.triggers ?? []);
      setAdvanced(wf.advanced ?? { concurrency: 1, timeout_minutes: 0 });
      setProjects(projectItems);
      setEnvironments(envItems);
    } catch (err) {
      setError(err instanceof BffError ? err.message : "加载工作流失败");
    } finally {
      setLoading(false);
    }
  }, [workspaceId, productId, workflowId]);

  useEffect(() => {
    void load();
  }, [load]);

  async function handleSave(activate: boolean) {
    setBusy(true);
    setError(null);
    try {
      const saved = await product.updateProductWorkflow(workspaceId, productId, workflowId, {
        name,
        description,
        status: activate ? "active" : "draft",
        stages,
        notifications,
        variables,
        triggers,
        advanced,
      });
      setStatus(saved.status);
      setStages(saved.stages?.length ? saved.stages : stages);
      setNotifications(saved.notifications ?? notifications);
      setVariables(saved.variables ?? variables);
      setTriggers(saved.triggers ?? triggers);
      setAdvanced(saved.advanced ?? advanced);
      if (activate) {
        router.push(`/products/workflows?productId=${productId}&workflowId=${workflowId}`);
      }
    } catch (err) {
      setError(err instanceof BffError ? err.message : "保存失败");
    } finally {
      setBusy(false);
    }
  }

  if (loading) {
    return (
      <div className="empty-state text-sm" role="status">
        加载工作流编辑器…
      </div>
    );
  }

  return (
    <ProductWorkflowEditor
      workspaceId={workspaceId}
      productId={productId}
      workflowId={workflowId}
      name={name}
      workflowKey={workflowKey}
      description={description}
      status={status}
      stages={stages}
      notifications={notifications}
      variables={variables}
      triggers={triggers}
      advanced={advanced}
      projects={projects}
      environments={environments}
      busy={busy}
      error={error}
      onNameChange={setName}
      onDescriptionChange={setDescription}
      onStagesChange={setStages}
      onNotificationsChange={setNotifications}
      onVariablesChange={setVariables}
      onTriggersChange={setTriggers}
      onAdvancedChange={setAdvanced}
      onSave={handleSave}
      onCancel={() => router.push(`/products/workflows?productId=${productId}`)}
    />
  );
}
