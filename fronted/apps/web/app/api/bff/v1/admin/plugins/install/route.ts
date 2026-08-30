import { BffError } from "@sparkops/contracts";
import { createBffHandler } from "@sparkops/bff-core";
import { installPlugin } from "@sparkops/bff-modules/plugins";

export const POST = createBffHandler(async (ctx, req) => {
  const formData = await req.formData();
  if (!(formData.get("file") instanceof File)) {
    throw new BffError("请选择插件 zip 文件", "INVALID_REQUEST", 400, ctx.traceId);
  }
  return installPlugin(ctx, formData);
});
