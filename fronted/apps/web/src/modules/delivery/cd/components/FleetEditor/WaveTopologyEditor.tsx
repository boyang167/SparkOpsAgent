"use client";

import { useMemo, useState } from "react";
import { InlineAlert } from "@/components/ui/InlineAlert";
import type { ClusterVM, FleetTopology, FleetVM } from "../../types";

interface WaveTopologyEditorProps {
  fleet: FleetVM;
  onChange: (fleet: FleetVM) => void;
}

export function WaveTopologyEditor({ fleet, onChange }: WaveTopologyEditorProps) {
  const [dragClusterId, setDragClusterId] = useState<string | null>(null);
  const clusterMap = useMemo(() => new Map(fleet.clusters.map((c) => [c.id, c])), [fleet.clusters]);

  const assigned = useMemo(() => {
    const set = new Set<string>();
    for (const wave of fleet.topology.waves) {
      for (const id of wave.clusters) set.add(id);
    }
    return set;
  }, [fleet.topology.waves]);

  const unassigned = fleet.clusters.filter((c) => !assigned.has(c.id));

  function patchTopology(topology: FleetTopology) {
    onChange({ ...fleet, topology });
  }

  function addWave() {
    patchTopology({
      waves: [...fleet.topology.waves, { name: `wave-${fleet.topology.waves.length + 1}`, clusters: [] }],
    });
  }

  function renameWave(index: number, name: string) {
    const waves = fleet.topology.waves.map((wave, i) => (i === index ? { ...wave, name } : wave));
    patchTopology({ waves });
  }

  function removeWave(index: number) {
    const removed = fleet.topology.waves[index];
    const waves = fleet.topology.waves.filter((_, i) => i !== index);
    if (removed && removed.clusters.length > 0) {
      const first = waves[0];
      if (first) {
        first.clusters = [...removed.clusters, ...first.clusters];
      } else {
        waves.push({ name: "default", clusters: removed.clusters });
      }
    }
    patchTopology({ waves });
  }

  function moveClusterToWave(clusterId: string, waveIndex: number) {
    const waves = fleet.topology.waves.map((wave) => ({
      ...wave,
      clusters: wave.clusters.filter((id) => id !== clusterId),
    }));
    waves[waveIndex] = {
      ...waves[waveIndex]!,
      clusters: [...waves[waveIndex]!.clusters, clusterId],
    };
    patchTopology({ waves });
  }

  function removeClusterFromWave(clusterId: string, waveIndex: number) {
    const waves = fleet.topology.waves.map((wave, index) =>
      index === waveIndex ? { ...wave, clusters: wave.clusters.filter((id) => id !== clusterId) } : wave,
    );
    patchTopology({ waves });
  }

  return (
    <div className="cd-fleet-editor stack-lg">
      <div className="cd-fleet-meta">
        <div className="cd-field">
          <label className="cd-field-label">Fleet 名称</label>
          <input className="input" value={fleet.name} onChange={(e) => onChange({ ...fleet, name: e.target.value })} />
        </div>
        <div className="cd-field">
          <label className="cd-field-label">描述</label>
          <input
            className="input"
            value={fleet.description}
            onChange={(e) => onChange({ ...fleet, description: e.target.value })}
          />
        </div>
      </div>

      <div className="cd-fleet-layout">
        <aside className="cd-fleet-pool">
          <h3 className="cd-side-title">可用集群</h3>
          <p className="field-hint">拖拽或点击分配到 Wave</p>
          <div className="cd-cluster-pool">
            {unassigned.map((cluster) => (
              <ClusterChip
                key={cluster.id}
                cluster={cluster}
                draggable
                onDragStart={() => setDragClusterId(cluster.id)}
                onDragEnd={() => setDragClusterId(null)}
              />
            ))}
            {unassigned.length === 0 ? <p className="muted text-sm">全部集群已分配到 Wave</p> : null}
          </div>
        </aside>

        <section className="cd-wave-board">
          <div className="cd-wave-board-header">
            <h3 className="cd-side-title">Wave 拓扑</h3>
            <button type="button" className="btn-secondary" onClick={addWave}>
              + 添加 Wave
            </button>
          </div>

          <div className="cd-wave-list">
            {fleet.topology.waves.map((wave, index) => (
              <div
                key={`${wave.name}-${index}`}
                className="cd-wave-lane"
                onDragOver={(e) => e.preventDefault()}
                onDrop={() => {
                  if (dragClusterId) {
                    moveClusterToWave(dragClusterId, index);
                    setDragClusterId(null);
                  }
                }}
              >
                <div className="cd-wave-lane-head">
                  <input
                    className="input cd-wave-name"
                    value={wave.name}
                    onChange={(e) => renameWave(index, e.target.value)}
                  />
                  <span className="muted text-sm">{wave.clusters.length} 集群</span>
                  <button type="button" className="btn-ghost text-sm" onClick={() => removeWave(index)}>
                    删除 Wave
                  </button>
                </div>
                <div className="cd-wave-clusters">
                  {wave.clusters.map((clusterId) => {
                    const cluster = clusterMap.get(clusterId);
                    if (!cluster) return null;
                    return (
                      <ClusterChip
                        key={clusterId}
                        cluster={cluster}
                        draggable
                        onDragStart={() => setDragClusterId(clusterId)}
                        onDragEnd={() => setDragClusterId(null)}
                        onRemove={() => removeClusterFromWave(clusterId, index)}
                      />
                    );
                  })}
                  {wave.clusters.length === 0 ? (
                    <p className="cd-wave-drop-hint">拖入集群，或从左侧点击分配</p>
                  ) : null}
                </div>
                {unassigned.length > 0 ? (
                  <div className="cd-wave-quick-add">
                    {unassigned.map((cluster) => (
                      <button
                        key={cluster.id}
                        type="button"
                        className="btn-ghost text-sm"
                        onClick={() => moveClusterToWave(cluster.id, index)}
                      >
                        + {cluster.name}
                      </button>
                    ))}
                  </div>
                ) : null}
              </div>
            ))}
          </div>
        </section>
      </div>

      <InlineAlert tone="info">
        发布时 Run Engine 将按 Wave 顺序执行：{fleet.topology.waves.map((w) => w.name).join(" → ") || "（未配置）"}
      </InlineAlert>
    </div>
  );
}

function ClusterChip({
  cluster,
  draggable,
  onDragStart,
  onDragEnd,
  onRemove,
}: {
  cluster: ClusterVM;
  draggable?: boolean;
  onDragStart?: () => void;
  onDragEnd?: () => void;
  onRemove?: () => void;
}) {
  return (
    <div
      className={`cd-cluster-chip${cluster.ready ? "" : " is-blocked"}`}
      draggable={draggable}
      onDragStart={onDragStart}
      onDragEnd={onDragEnd}
    >
      <div>
        <strong>{cluster.name}</strong>
        <span className="muted text-sm">
          {cluster.region} · {cluster.env}
        </span>
      </div>
      {!cluster.ready ? <span className="badge badge-warning">{cluster.reason ?? "未就绪"}</span> : null}
      {onRemove ? (
        <button type="button" className="btn-ghost text-sm" onClick={onRemove}>
          移出
        </button>
      ) : null}
    </div>
  );
}
