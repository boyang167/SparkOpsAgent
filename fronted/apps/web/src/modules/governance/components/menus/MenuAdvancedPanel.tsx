"use client";

import type { MenuButtonVM, MenuParamVM, RoleVM, UpsertMenuCommand } from "@sparkops/contracts/governance";
import { useState } from "react";

interface MenuAdvancedPanelProps {
  form: UpsertMenuCommand;
  roles: RoleVM[];
  onChange: (next: UpsertMenuCommand) => void;
}

function boolSelect(value: boolean, onChange: (v: boolean) => void) {
  return (
    <select className="input" value={value ? "1" : "0"} onChange={(e) => onChange(e.target.value === "1")}>
      <option value="0">否</option>
      <option value="1">是</option>
    </select>
  );
}

export function MenuAdvancedPanel({ form, roles, onChange }: MenuAdvancedPanelProps) {
  const [editingButtonIndex, setEditingButtonIndex] = useState<number | null>(null);
  const params = form.params ?? [];
  const buttons = form.buttons ?? [];

  function updateParams(next: MenuParamVM[]) {
    onChange({ ...form, params: next });
  }

  function updateButtons(next: MenuButtonVM[]) {
    onChange({ ...form, buttons: next });
  }

  function addParam() {
    updateParams([...params, { type: "query", key: "", value: "", sort: params.length + 1 }]);
  }

  function addButton() {
    updateButtons([
      ...buttons,
      { name: "", code: "", remark: "", sort: buttons.length + 1, role_ids: [] },
    ]);
  }

  return (
    <div className="stack-md">
      <section className="menu-form-section">
        <h3 className="menu-form-section-title">高级配置</h3>
        <div className="menu-advanced-grid">
          <label className="stack-xs">
            <span className="label">高亮菜单</span>
            <input
              className="input"
              value={form.active_menu ?? ""}
              onChange={(e) => onChange({ ...form, active_menu: e.target.value })}
              placeholder="路由 name，用于子页高亮父菜单"
            />
          </label>
          <label className="stack-xs">
            <span className="label">KeepAlive</span>
            {boolSelect(form.keep_alive ?? false, (keep_alive) => onChange({ ...form, keep_alive }))}
          </label>
          <label className="stack-xs">
            <span className="label">CloseTab</span>
            {boolSelect(form.close_tab ?? false, (close_tab) => onChange({ ...form, close_tab }))}
          </label>
          <label className="stack-xs">
            <span className="label">是否为基础页面</span>
            {boolSelect(form.is_basic ?? false, (is_basic) => onChange({ ...form, is_basic }))}
          </label>
          <label className="stack-xs menu-advanced-span-2">
            <span className="label">路由切换动画</span>
            <select
              className="input"
              value={form.transition_animation ?? "global"}
              onChange={(e) =>
                onChange({
                  ...form,
                  transition_animation: e.target.value as UpsertMenuCommand["transition_animation"],
                })
              }
            >
              <option value="global">跟随全局</option>
              <option value="fade">淡入淡出</option>
              <option value="none">无动画</option>
            </select>
          </label>
        </div>
      </section>

      <section className="menu-form-section">
        <div className="menu-section-head">
          <h3 className="menu-form-section-title !mb-0">菜单参数配置</h3>
          <button type="button" className="btn-secondary !py-1.5 !px-3 text-xs" onClick={addParam}>
            新增菜单参数
          </button>
        </div>
        <div className="table-wrap">
          <table className="data-table text-sm menu-config-table">
            <thead>
              <tr>
                <th>参数类型</th>
                <th>参数 key</th>
                <th>参数值</th>
                <th>操作</th>
              </tr>
            </thead>
            <tbody>
              {params.length === 0 ? (
                <tr>
                  <td colSpan={4} className="text-center text-muted py-6">
                    暂无数据
                  </td>
                </tr>
              ) : (
                params.map((param, index) => (
                  <tr key={param.id ?? `param-${index}`}>
                    <td>
                      <select
                        className="input !py-1.5"
                        value={param.type}
                        onChange={(e) => {
                          const next = [...params];
                          next[index] = {
                            ...param,
                            type: e.target.value as MenuParamVM["type"],
                          };
                          updateParams(next);
                        }}
                      >
                        <option value="query">query</option>
                        <option value="params">params</option>
                      </select>
                    </td>
                    <td>
                      <input
                        className="input !py-1.5 font-mono text-xs"
                        value={param.key}
                        onChange={(e) => {
                          const next = [...params];
                          next[index] = { ...param, key: e.target.value };
                          updateParams(next);
                        }}
                        placeholder="id"
                      />
                    </td>
                    <td>
                      <input
                        className="input !py-1.5 font-mono text-xs"
                        value={param.value ?? ""}
                        onChange={(e) => {
                          const next = [...params];
                          next[index] = { ...param, value: e.target.value };
                          updateParams(next);
                        }}
                        placeholder="value"
                      />
                    </td>
                    <td>
                      <button
                        type="button"
                        className="menu-tree-action menu-tree-action-danger"
                        onClick={() => updateParams(params.filter((_, i) => i !== index))}
                      >
                        删除
                      </button>
                    </td>
                  </tr>
                ))
              )}
            </tbody>
          </table>
        </div>
      </section>

      <section className="menu-form-section">
        <div className="menu-section-head">
          <div>
            <h3 className="menu-form-section-title !mb-0">可控按钮配置</h3>
            <p className="text-xs text-muted mt-1">为页面内按钮配置权限码，并绑定可使用的角色</p>
          </div>
          <button type="button" className="btn-secondary !py-1.5 !px-3 text-xs" onClick={addButton}>
            新增可控按钮
          </button>
        </div>
        <div className="table-wrap">
          <table className="data-table text-sm menu-config-table">
            <thead>
              <tr>
                <th>按钮名称</th>
                <th>权限码</th>
                <th>备注</th>
                <th>授权角色</th>
                <th>操作</th>
              </tr>
            </thead>
            <tbody>
              {buttons.length === 0 ? (
                <tr>
                  <td colSpan={5} className="text-center text-muted py-6">
                    暂无数据
                  </td>
                </tr>
              ) : (
                buttons.map((button, index) => (
                  <tr key={button.id ?? `btn-${index}`}>
                    <td>
                      <input
                        className="input !py-1.5"
                        value={button.name}
                        onChange={(e) => {
                          const next = [...buttons];
                          next[index] = { ...button, name: e.target.value };
                          updateButtons(next);
                        }}
                        placeholder="新增"
                      />
                    </td>
                    <td>
                      <input
                        className="input !py-1.5 font-mono text-xs"
                        value={button.code}
                        onChange={(e) => {
                          const next = [...buttons];
                          next[index] = { ...button, code: e.target.value };
                          updateButtons(next);
                        }}
                        placeholder="create"
                      />
                    </td>
                    <td>
                      <input
                        className="input !py-1.5"
                        value={button.remark ?? ""}
                        onChange={(e) => {
                          const next = [...buttons];
                          next[index] = { ...button, remark: e.target.value };
                          updateButtons(next);
                        }}
                        placeholder="备注"
                      />
                    </td>
                    <td>
                      <button
                        type="button"
                        className="btn-ghost !py-1 !px-2 text-xs"
                        onClick={() =>
                          setEditingButtonIndex(editingButtonIndex === index ? null : index)
                        }
                      >
                        {(button.role_ids ?? []).length > 0
                          ? `${button.role_ids!.length} 个角色`
                          : "分配角色"}
                      </button>
                      {editingButtonIndex === index ? (
                        <div className="menu-button-role-picker">
                          {roles.map((role) => {
                            const checked = (button.role_ids ?? []).includes(role.id);
                            return (
                              <label key={role.id} className="menu-button-role-option">
                                <input
                                  type="checkbox"
                                  checked={checked}
                                  onChange={() => {
                                    const next = [...buttons];
                                    const ids = new Set(button.role_ids ?? []);
                                    if (checked) ids.delete(role.id);
                                    else ids.add(role.id);
                                    next[index] = {
                                      ...button,
                                      role_ids: Array.from(ids),
                                    };
                                    updateButtons(next);
                                  }}
                                />
                                <span>{role.name}</span>
                              </label>
                            );
                          })}
                        </div>
                      ) : null}
                    </td>
                    <td>
                      <button
                        type="button"
                        className="menu-tree-action menu-tree-action-danger"
                        onClick={() => updateButtons(buttons.filter((_, i) => i !== index))}
                      >
                        删除
                      </button>
                    </td>
                  </tr>
                ))
              )}
            </tbody>
          </table>
        </div>
      </section>
    </div>
  );
}
