import { createBffContext, createTraceId, fail, ok } from "@sparkops/bff-core";
import { deleteMenu, getMenu, updateMenu } from "@sparkops/bff-modules/governance";
import { UpsertMenuCommandSchema } from "@sparkops/contracts/governance";
import { NextRequest } from "next/server";

type RouteParams = { params: Promise<{ wsId: string; menuId: string }> };

export const GET = async (req: NextRequest, { params }: RouteParams) => {
  const traceId = createTraceId(req);
  try {
    const ctx = await createBffContext(req);
    const { wsId, menuId } = await params;
    const menu = await getMenu(ctx, wsId, menuId);
    return ok(menu, traceId);
  } catch (err) {
    return fail(err, traceId);
  }
};

export const PATCH = async (req: NextRequest, { params }: RouteParams) => {
  const traceId = createTraceId(req);
  try {
    const ctx = await createBffContext(req);
    const { wsId, menuId } = await params;
    const body = UpsertMenuCommandSchema.parse(await req.json());
    await updateMenu(ctx, wsId, menuId, body);
    return ok({ updated: true }, traceId);
  } catch (err) {
    return fail(err, traceId);
  }
};

export const DELETE = async (req: NextRequest, { params }: RouteParams) => {
  const traceId = createTraceId(req);
  try {
    const ctx = await createBffContext(req);
    const { wsId, menuId } = await params;
    await deleteMenu(ctx, wsId, menuId);
    return ok({ deleted: true }, traceId);
  } catch (err) {
    return fail(err, traceId);
  }
};
