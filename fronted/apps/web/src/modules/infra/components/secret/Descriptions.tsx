import type { ReactNode } from "react";

interface DescriptionsProps {
  title: string;
  column?: 1 | 2;
  children: ReactNode;
}

interface DescriptionsItemProps {
  label: string;
  span?: 1 | 2;
  children: ReactNode;
}

export function Descriptions({ title, column = 2, children }: DescriptionsProps) {
  return (
    <section className={`secret-desc cols-${column}`}>
      <h4 className="secret-desc-title">{title}</h4>
      <div className="secret-desc-grid">{children}</div>
    </section>
  );
}

export function DescriptionsItem({ label, span = 1, children }: DescriptionsItemProps) {
  return (
    <>
      <div className="secret-desc-label">{label}</div>
      <div className={`secret-desc-value${span === 2 ? " is-span-value" : ""}`}>{children}</div>
    </>
  );
}
