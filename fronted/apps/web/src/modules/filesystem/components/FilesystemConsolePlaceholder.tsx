import { InlineAlert } from "@/components/ui/InlineAlert";

export function FilesystemConsolePlaceholder({
  title,
  description,
}: {
  title: string;
  description?: string;
}) {
  return (
    <div className="card stack-sm filesystem-console-placeholder">
      <h2 className="text-lg font-semibold">{title}</h2>
      <p className="text-sm text-muted">
        {description ?? "该能力将在后续版本接入统一文件基础设施。"}
      </p>
      <InlineAlert tone="info">当前为导航占位，业务 API 尚未在此入口开放。</InlineAlert>
    </div>
  );
}