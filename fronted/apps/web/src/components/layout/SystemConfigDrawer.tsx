"use client";

import { useMemo, useState } from "react";
import { Drawer } from "@/components/ui/Drawer";
import {
  PRIMARY_PRESETS,
  THEME_PRESETS,
  type CardStyle,
  type ThemeMode,
} from "./theme/theme-config";
import { useThemeConfig } from "./theme/ThemeProvider";

type ConfigTab = "appearance" | "layout" | "presets" | "general";

const TABS: Array<{ key: ConfigTab; label: string }> = [
  { key: "appearance", label: "外观" },
  { key: "layout", label: "布局" },
  { key: "presets", label: "预设" },
  { key: "general", label: "通用" },
];

interface SystemConfigDrawerProps {
  open: boolean;
  onClose: () => void;
}

export function SystemConfigDrawer({ open, onClose }: SystemConfigDrawerProps) {
  const { config, patchConfig, resetConfig, setMode } = useThemeConfig();
  const [tab, setTab] = useState<ConfigTab>("appearance");

  const tabs = useMemo(
    () => (
      <div className="config-tabs">
        {TABS.map((item) => (
          <button
            key={item.key}
            type="button"
            className={tab === item.key ? "config-tab is-active" : "config-tab"}
            onClick={() => setTab(item.key)}
          >
            {item.label}
          </button>
        ))}
      </div>
    ),
    [tab],
  );

  return (
    <Drawer
      open={open}
      onClose={onClose}
      title="系统配置"
      panelClassName="drawer-panel--config"
      tabs={tabs}
      footer={
        <div className="config-footer">
          <button type="button" className="btn-secondary" onClick={resetConfig}>
            重置配置
          </button>
          <button type="button" className="btn" onClick={onClose}>
            完成
          </button>
        </div>
      }
    >
      {tab === "appearance" ? (
        <div className="config-section stack-md">
          <section>
            <h3 className="config-section-title">主题模式</h3>
            <div className="config-segment">
              {(
                [
                  { key: "light", label: "浅色" },
                  { key: "dark", label: "深色" },
                  { key: "system", label: "跟随系统" },
                ] as const
              ).map((item) => (
                <button
                  key={item.key}
                  type="button"
                  className={config.mode === item.key ? "config-segment-btn is-active" : "config-segment-btn"}
                  onClick={() => setMode(item.key as ThemeMode)}
                >
                  {item.label}
                </button>
              ))}
            </div>
          </section>

          <section>
            <h3 className="config-section-title">主题色</h3>
            <div className="config-color-row">
              {PRIMARY_PRESETS.map((preset) => (
                <button
                  key={preset.value}
                  type="button"
                  className={
                    config.primaryColor.toLowerCase() === preset.value.toLowerCase()
                      ? "config-color-dot is-active"
                      : "config-color-dot"
                  }
                  style={{ background: preset.value }}
                  aria-label={preset.label}
                  onClick={() => patchConfig({ primaryColor: preset.value })}
                />
              ))}
              <label className="config-color-input">
                <input
                  type="color"
                  value={config.primaryColor}
                  onChange={(e) => patchConfig({ primaryColor: e.target.value })}
                />
                <span>{config.primaryColor}</span>
              </label>
            </div>
          </section>

          <section>
            <h3 className="config-section-title">外观细节</h3>
            <label className="config-row">
              <span>全局圆角</span>
              <div className="config-slider-wrap">
                <input
                  type="range"
                  min={0}
                  max={1}
                  step={0.05}
                  value={config.borderRadius}
                  onChange={(e) => patchConfig({ borderRadius: Number(e.target.value) })}
                />
                <span>{config.borderRadius.toFixed(2)}rem</span>
              </div>
            </label>
            <label className="config-row">
              <span>卡片样式</span>
              <div className="config-segment config-segment--compact">
                {(["border", "shadow"] as CardStyle[]).map((style) => (
                  <button
                    key={style}
                    type="button"
                    className={
                      config.cardStyle === style ? "config-segment-btn is-active" : "config-segment-btn"
                    }
                    onClick={() => patchConfig({ cardStyle: style })}
                  >
                    {style === "border" ? "边框" : "阴影"}
                  </button>
                ))}
              </div>
            </label>
          </section>
        </div>
      ) : null}

      {tab === "layout" ? (
        <div className="config-section stack-md">
          <section>
            <h3 className="config-section-title">菜单风格</h3>
            <label className="config-switch-row">
              <span>深色侧边栏</span>
              <input
                type="checkbox"
                checked={config.sidebarDark}
                onChange={(e) => patchConfig({ sidebarDark: e.target.checked })}
              />
            </label>
          </section>
          <section>
            <h3 className="config-section-title">标签栏</h3>
            <label className="config-switch-row">
              <span>展示图标</span>
              <input
                type="checkbox"
                checked={config.showTabIcons}
                onChange={(e) => patchConfig({ showTabIcons: e.target.checked })}
              />
            </label>
          </section>
        </div>
      ) : null}

      {tab === "presets" ? (
        <div className="config-section stack-sm">
          {THEME_PRESETS.map((preset) => (
            <button
              key={preset.name}
              type="button"
              className="config-preset-card"
              onClick={() => patchConfig(preset.config)}
            >
              <span className="font-medium">{preset.name}</span>
              <span className="text-sm text-muted">一键应用常用组合</span>
            </button>
          ))}
        </div>
      ) : null}

      {tab === "general" ? (
        <div className="config-section stack-md text-sm text-muted">
          <p>主题与布局配置保存在浏览器本地（localStorage），不会影响其他用户。</p>
          <p>快捷键：<kbd>Ctrl</kbd> + <kbd>K</kbd> 打开搜索。</p>
        </div>
      ) : null}
    </Drawer>
  );
}
