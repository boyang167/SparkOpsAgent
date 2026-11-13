import type { LifecycleRuleAction } from "@sparkops/contracts/filesystem";

export const LIFECYCLE_WHEN_KEYS = [
  "scope",
  "classification",
  "mime_prefix",
  "min_size",
  "max_age_days",
] as const;

export const LIFECYCLE_ACTIONS: LifecycleRuleAction[] = [
  "replicate",
  "archive",
  "trash",
  "purge",
  "rescan",
];

export const LIFECYCLE_SCOPE_VALUES = [
  "ORGANIZATION",
  "WORKSPACE",
  "PROJECT",
  "USER",
] as const;

export const LIFECYCLE_CLASSIFICATION_VALUES = [
  "PUBLIC",
  "INTERNAL",
  "CONFIDENTIAL",
  "RESTRICTED",
] as const;

export type LifecycleRuleDraft = {
  id: string;
  when: Record<string, string | number>;
  actions: LifecycleRuleAction[];
};

export function emptyLifecycleRules(): { rules: LifecycleRuleDraft[] } {
  return { rules: [] };
}

export function buildLifecycleRulesPayload(drafts: LifecycleRuleDraft[]) {
  return {
    rules: drafts.map((rule) => ({
      id: rule.id,
      when: rule.when,
      actions: rule.actions,
    })),
  };
}
