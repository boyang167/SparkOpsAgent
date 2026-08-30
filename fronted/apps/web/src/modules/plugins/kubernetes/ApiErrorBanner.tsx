import type { ApiErrorInfo } from "./api";

type ApiErrorBannerProps = {
  error: ApiErrorInfo | string | null;
};

export function ApiErrorBanner({ error }: ApiErrorBannerProps) {
  if (!error) return null;
  const info = typeof error === "string" ? { message: error } : error;
  return (
    <div className="alert alert-warning" role="alert">
      <div className="font-medium">
        {info.code ? `${info.code}: ` : ""}
        {info.message}
      </div>
      {info.status || info.traceId ? (
        <div className="text-sm mt-1">
          {info.status ? `HTTP ${info.status}` : ""}
          {info.status && info.traceId ? " · " : ""}
          {info.traceId ? `trace ${info.traceId}` : ""}
        </div>
      ) : null}
      {info.detail && info.detail !== info.message ? (
        <pre className="input-mono mt-2 whitespace-pre-wrap text-xs">{info.detail}</pre>
      ) : null}
    </div>
  );
}
