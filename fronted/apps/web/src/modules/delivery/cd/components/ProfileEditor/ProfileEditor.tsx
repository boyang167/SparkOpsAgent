"use client";

import { useMemo, useState } from "react";
import { InlineAlert } from "@/components/ui/InlineAlert";
import { parseYamlEditorDocument, stringifyYamlEditorDocument } from "@/modules/plugins/kubernetes/yaml-document";
import { getMockProfile } from "../../mock/data";
import type { ProfileVM } from "../../types";
import { YamlEditorPanel } from "../shared/YamlEditorPanel";

const PROFILE_FIELDS: Array<{
  key: string;
  label: string;
  type: "string" | "number";
  hint?: string;
}> = [
  { key: "replicas", label: "副本数", type: "number" },
  { key: "ingress_host", label: "Ingress 域名", type: "string" },
  { key: "log_level", label: "日志级别", type: "string" },
  { key: "cpu_request", label: "CPU Request", type: "string" },
  { key: "memory_request", label: "Memory Request", type: "string" },
  { key: "hpa_min", label: "HPA 最小副本", type: "number", hint: "生产环境可选" },
  { key: "hpa_max", label: "HPA 最大副本", type: "number", hint: "生产环境可选" },
];

interface ProfileEditorProps {
  profileId: string;
}

export function ProfileEditor({ profileId }: ProfileEditorProps) {
  const initial = useMemo(() => getMockProfile(profileId), [profileId]);
  const [profile, setProfile] = useState<ProfileVM | null>(initial ?? null);
  const [tab, setTab] = useState<"form" | "yaml">("form");
  const [yamlText, setYamlText] = useState(
    initial ? stringifyYamlEditorDocument({ values: initial.values, patch: initial.patch }) : "",
  );
  const [yamlError, setYamlError] = useState<string | null>(null);
  const [savedHint, setSavedHint] = useState<string | null>(null);

  if (!profile) {
    return <InlineAlert tone="error">未找到 Profile</InlineAlert>;
  }

  function updateValue(key: string, raw: string, type: "string" | "number") {
    const values = { ...profile!.values };
    if (raw === "") {
      delete values[key];
    } else {
      values[key] = type === "number" ? Number(raw) : raw;
    }
    setProfile({ ...profile!, values });
    setSavedHint(null);
  }

  function syncYamlToForm() {
    try {
      const parsed = parseYamlEditorDocument(yamlText) as {
        values?: ProfileVM["values"];
        patch?: ProfileVM["patch"];
      };
      setProfile({
        ...profile!,
        values: parsed.values ?? profile!.values,
        patch: parsed.patch ?? profile!.patch,
      });
      setYamlError(null);
      setSavedHint("已从 YAML 同步到表单（Mock）");
    } catch (err) {
      setYamlError(err instanceof Error ? err.message : "YAML 解析失败");
    }
  }

  return (
    <div className="cd-profile-editor stack-lg">
      <div className="cd-studio-toolbar">
        <div>
          <h2 className="cd-studio-title">{profile.name}</h2>
          <p className="muted text-sm">环境 {profile.env} · 配置「每环境参数」</p>
        </div>
        <div className="cd-studio-actions">
          <div className="cd-tab-switch">
            <button
              type="button"
              className={`cd-tab${tab === "form" ? " is-active" : ""}`}
              onClick={() => setTab("form")}
            >
              表单
            </button>
            <button
              type="button"
              className={`cd-tab${tab === "yaml" ? " is-active" : ""}`}
              onClick={() => {
                setYamlText(stringifyYamlEditorDocument({ values: profile.values, patch: profile.patch }));
                setTab("yaml");
              }}
            >
              YAML
            </button>
          </div>
          <button
            type="button"
            className="btn"
            onClick={() => setSavedHint(`已保存 Profile（Mock）· ${new Date().toLocaleTimeString()}`)}
          >
            保存
          </button>
        </div>
      </div>

      {savedHint ? <InlineAlert tone="success">{savedHint}</InlineAlert> : null}

      <div className="cd-field">
        <label className="cd-field-label">描述</label>
        <input
          className="input"
          value={profile.description}
          onChange={(e) => setProfile({ ...profile, description: e.target.value })}
        />
      </div>

      {tab === "form" ? (
        <div className="cd-profile-form section">
          <div className="section-header">
            <h3 className="section-title">环境参数 (L2 Profile)</h3>
          </div>
          <div className="cd-form-grid two-col">
            {PROFILE_FIELDS.map((field) => (
              <div key={field.key} className="cd-field">
                <label className="cd-field-label">{field.label}</label>
                <input
                  className="input"
                  type={field.type === "number" ? "number" : "text"}
                  value={
                    profile.values[field.key] === undefined || profile.values[field.key] === null
                      ? ""
                      : String(profile.values[field.key])
                  }
                  onChange={(e) => updateValue(field.key, e.target.value, field.type)}
                />
                {field.hint ? <p className="field-hint">{field.hint}</p> : null}
              </div>
            ))}
          </div>
        </div>
      ) : (
        <div className="cd-yaml-workspace">
          <div className="cd-yaml-toolbar">
            <p className="muted text-sm">values + patch 合并 YAML，用于环境差异覆盖。</p>
            <button type="button" className="btn-secondary" onClick={syncYamlToForm}>
              应用到表单
            </button>
          </div>
          <YamlEditorPanel value={yamlText} onChange={setYamlText} error={yamlError} />
        </div>
      )}
    </div>
  );
}
