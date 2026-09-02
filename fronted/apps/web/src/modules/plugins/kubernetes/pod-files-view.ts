export type PodFileEntry = {
  name: string;
  size: string;
  mode: string;
  user: string;
  group: string;
  modTime: string;
  isDir: boolean;
  isSymlink: boolean;
  link: string;
};

export function parseLsOutput(output: string): PodFileEntry[] {
  const files: PodFileEntry[] = [];
  for (const rawLine of output.split("\n")) {
    const line = rawLine.trim();
    if (!line || line.startsWith("total ")) continue;
    const entry = parseLsLine(line);
    if (!entry || entry.name === "." || entry.name === "..") continue;
    files.push(entry);
  }
  return files;
}

export function joinPodFilePath(current: string, name: string): string {
  const trimmed = name.trim();
  if (!trimmed) return current || "/";
  if (current === "/" || current === "") return `/${trimmed}`;
  return `${current.replace(/\/+$/, "")}/${trimmed}`;
}

export function podFileBreadcrumbs(currentPath: string): string[] {
  if (!currentPath || currentPath === "/") return [];
  return currentPath.split("/").filter(Boolean);
}

export function podFilePathFromBreadcrumbs(segments: string[]): string {
  if (segments.length === 0) return "/";
  return `/${segments.join("/")}`;
}

function parseLsLine(line: string): PodFileEntry | null {
  const parts = line.trim().split(/\s+/);
  if (parts.length < 9) return null;

  const mode = parts[0];
  const user = parts[2];
  const group = parts[3];

  let index = 4;
  let size = parts[index];
  if (size.endsWith(",") && parts[index + 1] && /^\d+$/.test(parts[index + 1])) {
    size = `${size} ${parts[index + 1]}`;
    index += 1;
  }
  index += 1;

  const month = parts[index];
  const day = parts[index + 1];
  const clock = parts[index + 2];
  if (!month || !day || !clock) return null;
  const modTime = `${month} ${day} ${clock}`;
  index += 3;

  const firstNameToken = parts[index];
  if (!firstNameToken) return null;

  let remainder = extractTail(line, firstNameToken);
  let name = remainder;
  let link = "";
  const arrowIndex = remainder.indexOf(" -> ");
  if (mode.startsWith("l") && arrowIndex >= 0) {
    name = remainder.slice(0, arrowIndex).trim();
    link = remainder.slice(arrowIndex + 4).trim();
  }

  return {
    name,
    size,
    mode,
    user,
    group,
    modTime,
    isDir: mode.startsWith("d"),
    isSymlink: mode.startsWith("l"),
    link,
  };
}

function extractTail(line: string, token: string): string {
  const index = line.lastIndexOf(token);
  if (index < 0) return token;
  return line.slice(index).trim();
}
