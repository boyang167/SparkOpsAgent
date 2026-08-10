import {
  AUTH_COOKIE,
  createBffContext,
  createTraceId,
  fail,
  ok,
} from "@sparkops/bff-core";
import { executeLDAPLogin } from "@sparkops/bff-modules/identity";
import { LDAPLoginCommandSchema } from "@sparkops/contracts/identity";
import { NextRequest } from "next/server";

export async function POST(req: NextRequest) {
  const traceId = createTraceId(req);
  try {
    const ctx = await createBffContext(req);
    const body = LDAPLoginCommandSchema.parse(await req.json());
    const session = await executeLDAPLogin(ctx, body);
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
