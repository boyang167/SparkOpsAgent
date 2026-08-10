import {
  createBffContext,
  createTraceId,
  fail,
  ok,
  setWorkspaceCookie,
} from "@sparkops/bff-core";
import { SelectWorkspaceCommandSchema } from "@sparkops/contracts/tenant";
import { NextRequest } from "next/server";

export async function POST(req: NextRequest) {
  const traceId = createTraceId(req);
  try {
    const ctx = await createBffContext(req);
    const body = SelectWorkspaceCommandSchema.parse(await req.json());
    const res = ok({ workspaceId: body.workspaceId }, traceId);
    setWorkspaceCookie(res, body.workspaceId);
    return res;
  } catch (err) {
    return fail(err, traceId);
  }
}
