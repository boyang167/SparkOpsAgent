import {
  AUTH_COOKIE,
  createBffContext,
  createTraceId,
  fail,
  ok,
} from "@sparkops/bff-core";
import { completeOIDCLogin } from "@sparkops/bff-modules/identity";
import { NextRequest } from "next/server";

export async function GET(req: NextRequest) {
  const traceId = createTraceId(req);
  try {
    const ctx = await createBffContext(req);
    const code = req.nextUrl.searchParams.get("code");
    const state = req.nextUrl.searchParams.get("state");
    if (!code || !state) {
      return fail(new Error("missing code or state"), traceId);
    }
    const session = await completeOIDCLogin(ctx, code, state);
    const res = ok(
      { memberId: session.memberId, email: session.email, displayName: session.displayName },
      traceId,
    );
    res.cookies.set(AUTH_COOKIE, session.token, {
      httpOnly: true,
      sameSite: "lax",
      secure: process.env.NODE_ENV === "production",
      path: "/",
      maxAge: 60 * 60 * 24,
    });
    return res;
  } catch (err) {
    return fail(err, traceId);
  }
}
