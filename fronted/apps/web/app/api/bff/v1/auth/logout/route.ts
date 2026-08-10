import { clearAuthCookie, clearWorkspaceCookie, createTraceId, fail, ok } from "@sparkops/bff-core";
import { NextRequest, NextResponse } from "next/server";

function clearSessionCookies(res: NextResponse) {
  clearAuthCookie(res);
  clearWorkspaceCookie(res);
}

export async function GET(req: NextRequest) {
  const res = NextResponse.redirect(new URL("/login", req.url));
  clearSessionCookies(res);
  return res;
}

export async function POST(req: NextRequest) {
  const traceId = createTraceId(req);
  try {
    const res = ok({ ok: true as const }, traceId);
    clearSessionCookies(res);
    return res;
  } catch (err) {
    return fail(err, traceId);
  }
}
