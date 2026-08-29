import { createBffContext, createTraceId, fail, ok } from "@sparkops/bff-core";
import { getGitFileContent } from "@sparkops/bff-modules/workspace";
import { GetGitFileContentCommandSchema } from "@sparkops/contracts/workspace";
import { NextRequest } from "next/server";

export async function POST(
  req: NextRequest,
  context: { params: Promise<{ wsId: string }> },
) {
  const traceId = createTraceId(req);
  try {
    const ctx = await createBffContext(req);
    const params = await context.params;
    const body = GetGitFileContentCommandSchema.parse(await req.json());
    const data = await getGitFileContent(ctx, params.wsId, body);
    return ok(data, traceId);
  } catch (err) {
    return fail(err, traceId);
  }
}
