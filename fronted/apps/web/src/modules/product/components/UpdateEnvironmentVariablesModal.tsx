"use client";

import { useEffect, useState } from "react";
import { product } from "@sparkops/web-sdk";
import { BffError } from "@sparkops/contracts";
import type { EnvironmentVM } from "@sparkops/contracts/product";
import { InlineAlert } from "@/components/ui/InlineAlert";
import { Modal } from "@/components/ui/Modal";
import { rowsToVariables, variablesToRows, type VariableRow } from "./environment-variables";

interface UpdateEnvironmentVariablesModalProps {
  open: boolean;
  workspaceId: string;
  productId: string;
  environment: EnvironmentVM | null;
  onClose: () => void;
  onSaved: (variables: Record<string, string>) => void;
}

const FORM_ID = "update-env-variables-form";

export function UpdateEnvironmentVariablesModal({
  open,
  workspaceId,
  productId,
  environment,
  onClose,
  onSaved,
}: UpdateEnvironmentVariablesModalProps) {
  const [rows, setRows] = useState<VariableRow[]>([{ key: "", value: "" }]);
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);

  useEffect(() => {
    if (!open) return;
    setRows(variablesToRows(environment?.variables));
    setError(null);
    setBusy(false);
  }, [open, environment]);

  function updateRow(index: number, patch: Partial<VariableRow>) {
    setRows((prev) => prev.map((row, i) => (i === index ? { ...row, ...patch } : row)));
  }

  async function onSubmit(e: React.FormEvent) {
    e.preventDefault();
    if (!environment) return;
    const parsed = rowsToVariables(rows);
    if (parsed.error) {
      setError(parsed.error);
      return;
    }
    setBusy(true);
    setError(null);
    try {
      await product.updateEnvironment(workspaceId, productId, environment.id, {
        variables: parsed.variables,
      });
      onSaved(parsed.variables);
    } catch (err) {
      setError(err instanceof BffError ? err.message : "保存全局变量失败");
    } finally {
      setBusy(false);
    }
  }

  return (
    <Modal
      open={open}
      onClose={onClose}
      title="更新全局变量"
      description="这些变量作用于当前环境，可在服务 YAML 中引用以实现多环境差异化配置。"
      size="lg"
      panelClassName="product-env-var-modal"
      footer={
        <div className="modal-footer-actions">
          <button type="button" className="btn-secondary" disabled={busy} onClick={onClose}>
            取消
          </button>
          <button type="submit" form={FORM_ID} className="btn btn-primary" disabled={busy || !environment}>
            {busy ? "保存中…" : "保存"}
          </button>
        </div>
      }
    >
      <form id={FORM_ID} className="product-env-var-form" onSubmit={(e) => void onSubmit(e)}>
        {error ? <InlineAlert tone="error">{error}</InlineAlert> : null}
        <div className="product-env-var-head">
          <span>变量名</span>
          <span>变量值</span>
          <span className="sr-only">操作</span>
        </div>
        <div className="product-env-var-list">
          {rows.map((row, index) => (
            <div key={index} className="product-env-var-row">
              <input
                className="input"
                value={row.key}
                onChange={(e) => updateRow(index, { key: e.target.value })}
                placeholder="domain"
                autoComplete="off"
                spellCheck={false}
                disabled={busy}
              />
              <input
                className="input"
                value={row.value}
                onChange={(e) => updateRow(index, { value: e.target.value })}
                placeholder="vote-dev.example.com"
                autoComplete="off"
                spellCheck={false}
                disabled={busy}
              />
              <button
                type="button"
                className="btn-ghost text-sm"
                disabled={busy || rows.length <= 1}
                onClick={() => setRows((prev) => (prev.length <= 1 ? prev : prev.filter((_, i) => i !== index)))}
              >
                删除
              </button>
            </div>
          ))}
        </div>
        <button
          type="button"
          className="btn-ghost text-sm product-env-var-add"
          disabled={busy}
          onClick={() => setRows((prev) => [...prev, { key: "", value: "" }])}
        >
          添加变量
        </button>
      </form>
    </Modal>
  );
}
