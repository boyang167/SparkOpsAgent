interface ComingSoonPanelProps {
  title: string;
  description: string;
  features?: string[];
}

export function ComingSoonPanel({ title, description, features }: ComingSoonPanelProps) {
  return (
    <div className="section stack-md">
      <div>
        <h2 className="section-title">{title}</h2>
        <p className="text-muted mt-1">{description}</p>
      </div>
      {features && features.length > 0 ? (
        <ul className="text-sm text-muted list-disc pl-5 space-y-1">
          {features.map((feature) => (
            <li key={feature}>{feature}</li>
          ))}
        </ul>
      ) : null}
      <p className="text-xs text-muted">该功能参照 gin-vue-admin 设计，后续版本逐步接入。</p>
    </div>
  );
}
