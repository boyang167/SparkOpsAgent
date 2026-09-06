"use client";

import Link from "next/link";
import { useState } from "react";
import { InlineAlert } from "@/components/ui/InlineAlert";
import { getMockFleet } from "../../mock/data";
import type { FleetVM } from "../../types";
import { WaveTopologyEditor } from "./WaveTopologyEditor";

interface FleetEditorPageProps {
  fleetId: string;
}

export function FleetEditorPage({ fleetId }: FleetEditorPageProps) {
  const initial = getMockFleet(fleetId);
  const [fleet, setFleet] = useState<FleetVM | null>(initial ?? null);
  const [savedHint, setSavedHint] = useState<string | null>(null);

  if (!fleet) {
    return (
      <InlineAlert tone="error">
        未找到 Fleet。{" "}
        <Link href="/delivery/fleets" className="link-inline">
          返回列表
        </Link>
      </InlineAlert>
    );
  }

  return (
    <div className="stack-lg">
      <div className="cd-studio-toolbar">
        <div>
          <h2 className="cd-studio-title">{fleet.name}</h2>
          <p className="muted text-sm">配置「发到哪些集群、分几波」</p>
        </div>
        <button
          type="button"
          className="btn"
          onClick={() => setSavedHint(`已保存 Fleet（Mock）· ${new Date().toLocaleTimeString()}`)}
        >
          保存
        </button>
      </div>
      {savedHint ? <InlineAlert tone="success">{savedHint}</InlineAlert> : null}
      <WaveTopologyEditor fleet={fleet} onChange={setFleet} />
    </div>
  );
}
