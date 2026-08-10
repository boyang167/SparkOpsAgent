interface InlineAlertProps {
  tone: "success" | "error" | "warning" | "info";
  children: React.ReactNode;
}

const toneClass = {
  success: "inline-alert-success",
  error: "inline-alert-error",
  warning: "inline-alert-warning",
  info: "inline-alert-info",
} as const;

export function InlineAlert({ tone, children }: InlineAlertProps) {
  return <div className={`inline-alert ${toneClass[tone]}`}>{children}</div>;
}
