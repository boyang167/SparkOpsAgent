interface MetricCardProps {
  label: string;
  value: string | number;
  hint?: string;
  icon: React.ReactNode;
  tone?: "sky" | "violet" | "emerald" | "amber";
}

const toneClass: Record<NonNullable<MetricCardProps["tone"]>, string> = {
  sky: "metric-card-sky",
  violet: "metric-card-violet",
  emerald: "metric-card-emerald",
  amber: "metric-card-amber",
};

export function MetricCard({ label, value, hint, icon, tone = "sky" }: MetricCardProps) {
  return (
    <div className={`metric-card ${toneClass[tone]}`}>
      <div className="metric-card-icon" aria-hidden>
        {icon}
      </div>
      <div className="metric-card-body">
        <p className="metric-card-label">{label}</p>
        <p className="metric-card-value">{value}</p>
        {hint ? <p className="metric-card-hint">{hint}</p> : null}
      </div>
    </div>
  );
}
