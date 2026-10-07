"use client";

const HOOK_EVENTS = [
  { key: "finish_planning", label: "完成规划", hint: "规划提交后，等待外部系统确认再继续审批/执行" },
  { key: "before_execute", label: "执行前", hint: "审批通过后、启动发布项前触发" },
  { key: "all_done", label: "全部完成", hint: "所有发布项完成后，外部确认再标记成功" },
] as const;

export interface HookEventFormValue {
  enabled: boolean;
  url: string;
  description: string;
}

export interface HookSettingsFormValues {
  enabled: boolean;
  events: Record<string, HookEventFormValue>;
}

interface ReleasePlanHookSettingsFieldProps {
  values: HookSettingsFormValues;
  onChange: (values: HookSettingsFormValues) => void;
}

export function emptyHookSettingsFormValues(): HookSettingsFormValues {
  return {
    enabled: false,
    events: Object.fromEntries(
      HOOK_EVENTS.map((event) => [event.key, { enabled: false, url: "", description: "" }]),
    ),
  };
}

export function readHookSettingsFormValues(
  settings?: { enabled?: boolean; events?: Record<string, { enabled?: boolean; url?: string; description?: string }> },
): HookSettingsFormValues {
  const base = emptyHookSettingsFormValues();
  if (!settings) return base;
  base.enabled = settings.enabled ?? false;
  for (const event of HOOK_EVENTS) {
    const cfg = settings.events?.[event.key];
    if (!cfg) continue;
    base.events[event.key] = {
      enabled: cfg.enabled ?? false,
      url: cfg.url ?? "",
      description: cfg.description ?? "",
    };
  }
  return base;
}

export function buildHookSettingsPayload(values: HookSettingsFormValues) {
  const events: Record<string, { enabled: boolean; url?: string; description?: string }> = {};
  for (const event of HOOK_EVENTS) {
    const cfg = values.events[event.key];
    if (!cfg?.enabled) continue;
    events[event.key] = {
      enabled: true,
      url: cfg.url.trim() || undefined,
      description: cfg.description.trim() || undefined,
    };
  }
  return { enabled: values.enabled, events };
}

export function ReleasePlanHookSettingsField({ values, onChange }: ReleasePlanHookSettingsFieldProps) {
  function patchEvent(key: string, patch: Partial<HookEventFormValue>) {
    onChange({
      ...values,
      events: {
        ...values.events,
        [key]: { ...values.events[key], ...patch },
      },
    });
  }

  return (
    <div className="stack-sm">
      <label className="inline-flex items-center gap-2 text-sm">
        <input
          type="checkbox"
          checked={values.enabled}
          onChange={(e) => onChange({ ...values, enabled: e.target.checked })}
        />
        启用外部系统 Hook
      </label>
      {values.enabled ? (
        <div className="stack-sm pl-1">
          {HOOK_EVENTS.map((event) => {
            const cfg = values.events[event.key] ?? { enabled: false, url: "", description: "" };
            return (
              <div key={event.key} className="release-plan-hook-event-card">
                <label className="inline-flex items-center gap-2 text-sm font-medium">
                  <input
                    type="checkbox"
                    checked={cfg.enabled}
                    onChange={(e) => patchEvent(event.key, { enabled: e.target.checked })}
                  />
                  {event.label}
                </label>
                <p className="text-xs text-muted-foreground">{event.hint}</p>
                {cfg.enabled ? (
                  <div className="stack-xs mt-2">
                    <input
                      className="input input-sm w-full"
                      placeholder="Webhook URL（POST）"
                      value={cfg.url}
                      onChange={(e) => patchEvent(event.key, { url: e.target.value })}
                    />
                    <input
                      className="input input-sm w-full"
                      placeholder="说明（可选）"
                      value={cfg.description}
                      onChange={(e) => patchEvent(event.key, { description: e.target.value })}
                    />
                  </div>
                ) : null}
              </div>
            );
          })}
        </div>
      ) : null}
    </div>
  );
}
