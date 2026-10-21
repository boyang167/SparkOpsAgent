import { BffError } from "@sparkops/contracts";
import { createBffHandler } from "@sparkops/bff-core";
import { installPlugin } from "@sparkops/bff-modules/plugins";

export const POST = createBffHandler(async (ctx, req) => {
  let formData: FormData;
  try {
    formData = await req.formData();
  } catch {
    throw new BffError(
      "上传失败：请求体无效或过大（单文件不超过 50 MiB）",
      "INVALID_REQUEST",
      400,
      ctx.traceId,
    );
  }
  const file = formData.get("file");
  if (!(file instanceof File)) {
    throw new BffError("请选择插件 zip 文件", "INVALID_REQUEST", 400, ctx.traceId);
  }
  const outbound = new FormData();
  outbound.set("file", file, file.name || "plugin.zip");
  return installPlugin(ctx, outbound);
});
