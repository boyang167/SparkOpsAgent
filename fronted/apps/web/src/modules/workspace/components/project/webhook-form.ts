export const WEBHOOK_EVENT_OPTIONS = [
  { key: "Build", label: "Build-构建" },
  { key: "Deploy", label: "Deploy-发布" },
  { key: "Rollback", label: "Rollback-回滚" },
] as const;

const NAME_MAX = 255;
const URL_PATTERN = /^https?:\/\/[^\s]+$/i;

export function isValidWebhookName(name: string): boolean {
  const value = name.trim();
  return value.length > 0 && value.length <= NAME_MAX;
}

export function isValidWebhookURL(url: string): boolean {
  return URL_PATTERN.test(url.trim());
}

export function toggleWebhookEvent(selected: string[], key: string): string[] {
  if (selected.includes(key)) {
    return selected.filter((item) => item !== key);
  }
  return [...selected, key];
}

export function moveWebhookEvents(source: string[], keys: string[], toSelected: boolean): string[] {
  if (toSelected) {
    const next = [...source];
    for (const key of keys) {
      if (!next.includes(key)) next.push(key);
    }
    return next;
  }
  return source.filter((item) => !keys.includes(item));
}
