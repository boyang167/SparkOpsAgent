"use client";

import Link from "next/link";
import { cloneMockProfiles } from "../../mock/data";

export function ProfileListPanel() {
  const profiles = cloneMockProfiles();

  return (
    <section className="section stack-md">
      <div className="section-header">
        <div>
          <h2 className="section-title">Environment Profile</h2>
          <p className="muted text-sm">配置环境差异参数 — L2 Profile</p>
        </div>
      </div>
      <table className="data-table">
        <thead>
          <tr>
            <th>名称</th>
            <th>环境</th>
            <th>副本数</th>
            <th>Ingress</th>
            <th />
          </tr>
        </thead>
        <tbody>
          {profiles.map((profile) => (
            <tr key={profile.id}>
              <td>
                <strong>{profile.name}</strong>
                <p className="muted text-sm">{profile.description}</p>
              </td>
              <td>{profile.env}</td>
              <td>{String(profile.values.replicas ?? "—")}</td>
              <td>{String(profile.values.ingress_host ?? "—")}</td>
              <td>
                <Link href={`/delivery/profiles/${profile.id}`} className="link-inline">
                  编辑
                </Link>
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </section>
  );
}
