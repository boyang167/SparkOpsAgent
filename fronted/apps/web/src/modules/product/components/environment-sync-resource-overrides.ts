export function environmentSyncResourceKey(kind: string, name: string): string {
  return `${kind.trim()}/${name.trim()}`;
}

export type EnvironmentSyncResourceOverrides = Record<string, string>;
