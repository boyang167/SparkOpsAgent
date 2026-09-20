/** 去掉路径里因空格/`%20` 产生的空段，例如 `/%20/products` → `/products` */
export function cleanedPathname(pathname: string): string | null {
  let decoded = pathname;
  try {
    decoded = decodeURIComponent(pathname);
  } catch {
    decoded = pathname;
  }
  const next = `/${decoded
    .split("/")
    .map((segment) => segment.trim())
    .filter(Boolean)
    .join("/")}`;
  return next === pathname ? null : next;
}
