"use client";

import Link from "next/link";
import { cloneMockFleets } from "../../mock/data";
import type { FleetWave } from "../../types";

export function FleetListPanel() {
  const fleets = cloneMockFleets();

  return (
    <section className="section stack-md">
      <div className="section-header">
        <div>
          <h2 className="section-title">Fleet 集群组</h2>
          <p className="muted text-sm">配置 Wave 拓扑 —「发到哪些集群、分几波」</p>
        </div>
      </div>
      <div className="cd-list-cards">
        {fleets.map((fleet) => (
          <article key={fleet.id} className="cd-list-card">
            <div>
              <h3>{fleet.name}</h3>
              <p className="muted text-sm">{fleet.description}</p>
              <p className="text-sm">
                {fleet.clusters.length} 集群 · {fleet.topology.waves.length} 波次 ·{" "}
                {fleet.topology.waves.map((wave: FleetWave) => wave.name).join(" → ")}
              </p>
            </div>
            <Link href={`/delivery/fleets/${fleet.id}`} className="btn-secondary">
              编辑拓扑
            </Link>
          </article>
        ))}
      </div>
    </section>
  );
}
