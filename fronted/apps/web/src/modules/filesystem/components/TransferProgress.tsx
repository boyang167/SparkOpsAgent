import type { TransferProgress as TransferProgressVM } from "@sparkops/web-sdk/filesystem";

interface TransferProgressProps {
  progress: TransferProgressVM;
  label?: string;
}

export function TransferProgress({ progress, label = "上传进度" }: TransferProgressProps) {
  const percent =
    progress.totalBytes > 0
      ? Math.min(100, Math.round((progress.uploadedBytes / progress.totalBytes) * 100))
      : 0;

  return (
    <div className="stack-sm">
      <div className="flex items-center justify-between text-sm text-muted">
        <span>{label}</span>
        <span>
          {percent}% ({progress.completedParts}/{progress.totalParts} 分片)
        </span>
      </div>
      <div
        className="h-2 rounded-full bg-surface-2 overflow-hidden"
        role="progressbar"
        aria-label={label}
        aria-valuemin={0}
        aria-valuemax={100}
        aria-valuenow={percent}
      >
        <div
          className="filesystem-progress-fill h-full bg-primary"
          style={{ width: `${percent}%` }}
        />
      </div>
    </div>
  );
}
