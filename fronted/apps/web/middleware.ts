import { NextResponse } from "next/server";
import type { NextRequest } from "next/server";
import { cleanedPathname } from "@/lib/normalize-pathname";

const publicPaths = ["/login", "/register", "/callback"];

export function middleware(req: NextRequest) {
  const { pathname } = req.nextUrl;
  const normalized = cleanedPathname(pathname);
  if (normalized) {
    const url = req.nextUrl.clone();
    url.pathname = normalized;
    return NextResponse.redirect(url);
  }

  if (publicPaths.some((path) => pathname.startsWith(path))) {
    return NextResponse.next();
  }
  if (pathname.startsWith("/api/bff")) {
    return NextResponse.next();
  }

  const token = req.cookies.get("sparkops_token")?.value;
  if (!token) {
    return NextResponse.redirect(new URL("/login", req.url));
  }

  const requestHeaders = new Headers(req.headers);
  requestHeaders.set("x-pathname", pathname);
  return NextResponse.next({ request: { headers: requestHeaders } });
}

export const config = {
  // Exclude BFF API routes so large multipart uploads (plugin ZIP ≤50MiB)
  // are not truncated by middleware's default ~10MiB body limit.
  matcher: ["/((?!_next/static|_next/image|favicon.ico|api/bff).*)"],
};
