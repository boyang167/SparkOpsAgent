import { NextResponse } from "next/server";
import type { NextRequest } from "next/server";

const publicPaths = ["/login", "/register", "/callback"];

export function middleware(req: NextRequest) {
  const { pathname } = req.nextUrl;

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

  return NextResponse.next();
}

export const config = {
  matcher: ["/((?!_next/static|_next/image|favicon.ico).*)"],
};
