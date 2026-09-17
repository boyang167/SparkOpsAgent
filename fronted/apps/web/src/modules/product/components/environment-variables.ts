export type VariableRow = { key: string; value: string };

export function variablesToRows(variables?: Record<string, string> | null): VariableRow[] {
  const entries = Object.entries(variables ?? {}).map(([key, value]) => ({ key, value }));
  entries.sort((a, b) => a.key.localeCompare(b.key));
  return entries.length > 0 ? entries : [{ key: "", value: "" }];
}

export function rowsToVariables(rows: VariableRow[]): { variables: Record<string, string>; error: string | null } {
  const variables: Record<string, string> = {};
  for (const row of rows) {
    const key = row.key.trim();
    if (!key) {
      if (row.value.trim()) {
        return { variables: {}, error: "变量名不能为空" };
      }
      continue;
    }
    if (!/^[A-Za-z_][A-Za-z0-9_]*$/.test(key)) {
      return { variables: {}, error: `变量名「${key}」仅支持字母、数字和下划线，且不能以数字开头` };
    }
    if (key in variables) {
      return { variables: {}, error: `变量名「${key}」重复` };
    }
    variables[key] = row.value;
  }
  return { variables, error: null };
}
