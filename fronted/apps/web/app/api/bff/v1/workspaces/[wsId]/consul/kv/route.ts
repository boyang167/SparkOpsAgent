import { createBffHandler } from "@sparkops/bff-core";
import { deleteConsulKV, listConsulKV, putConsulKV } from "@sparkops/bff-modules/cluster";

export const GET = createBffHandler(async (ctx, req, { params }) => {
  const wsId = params?.wsId ?? ctx.workspaceId;
  if (!wsId) throw new Error("workspace required");
  const prefix = req.nextUrl.searchParams.get("prefix") ?? undefined;
  return listConsulKV(ctx, wsId, prefix);
});

export const PUT = createBffHandler(async (ctx, req, { params }) => {
  const wsId = params?.wsId ?? ctx.workspaceId;
  if (!wsId) throw new Error("workspace required");
  const body = (await req.json()) as { key: string; value: string };
  return putConsulKV(ctx, wsId, body);
});

export const DELETE = createBffHandler(async (ctx, req, { params }) => {
  const wsId = params?.wsId ?? ctx.workspaceId;
  if (!wsId) throw new Error("workspace required");
  const key = req.nextUrl.searchParams.get("key");
  if (!key) throw new Error("key required");
  return deleteConsulKV(ctx, wsId, key);
});
