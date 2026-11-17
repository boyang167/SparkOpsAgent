import type { BffContext } from "@sparkops/bff-core";
import { BffError } from "@sparkops/contracts";
import { FileContentAccessVMSchema } from "@sparkops/contracts/filesystem";

function previewUnavailableMessage(
  reason: string | undefined,
): string {
  switch (reason) {
    case "PREVIEW_TOO_LARGE":
      return "文件体积超过在线预览上限，请下载后查看。";
    case "PREVIEW_GATEWAY_UNAVAILABLE":
      return "传输网关未配置或未启动，无法预览文件内容。";
    case "PREVIEW_UNSUPPORTED":
      return "此文件类型暂不支持在线预览。";
    default:
      return "当前无法预览该文件。";
  }
}

export type LoadedFilePreview = {
  body: Uint8Array;
  mimeType: string;
  previewKind: string;
};

export async function loadFilePreviewContent(
  ctx: BffContext,
  workspaceId: string,
  nodeId: string,
  query: { revision_id?: string; namespace_id?: string } = {},
): Promise<LoadedFilePreview> {
  const raw = await ctx.upstream.filesystem.getFileContentAccess(workspaceId, nodeId, query);
  const parsed = FileContentAccessVMSchema.safeParse(raw);
  if (!parsed.success) {
    throw new BffError(
      "预览元数据校验失败",
      "PREVIEW_CONTRACT_INVALID",
      502,
      ctx.traceId,
    );
  }
  const access = parsed.data;
  if (!access.preview_supported || !access.download_url || !access.capability_token) {
    throw new BffError(
      previewUnavailableMessage(access.reason_code),
      access.reason_code ?? "PREVIEW_UNAVAILABLE",
      422,
      ctx.traceId,
    );
  }

  let response: Response;
  try {
    response = await fetch(access.download_url, {
      method: "GET",
      headers: { Authorization: `Bearer ${access.capability_token}` },
      cache: "no-store",
    });
  } catch {
    throw new BffError(
      "无法连接传输网关拉取文件内容，请确认网关已启动。",
      "PREVIEW_GATEWAY_UNREACHABLE",
      503,
      ctx.traceId,
    );
  }
  if (!response.ok) {
    throw new BffError(
      `文件内容下载失败（HTTP ${response.status}）`,
      "PREVIEW_DOWNLOAD_FAILED",
      response.status === 404 ? 404 : 502,
      ctx.traceId,
    );
  }

  const body = new Uint8Array(await response.arrayBuffer());
  if (access.size > 0 && body.byteLength > access.size) {
    throw new BffError("下载内容长度异常", "PREVIEW_DOWNLOAD_INVALID", 502, ctx.traceId);
  }

  return {
    body,
    mimeType: access.mime_type || "application/octet-stream",
    previewKind: access.preview_kind,
  };
}