export function formatDeploymentLogText(raw: string): string {
  const lines: string[] = [];
  for (const line of raw.split("\n")) {
    const trimmed = line.trim();
    if (!trimmed) continue;
    try {
      const payload = JSON.parse(trimmed) as { message?: string };
      if (typeof payload.message === "string" && payload.message.trim()) {
        lines.push(payload.message.trim());
        continue;
      }
    } catch {
      // fall through — treat raw line as message
    }
    lines.push(trimmed);
  }
  return lines.join("\n");
}
