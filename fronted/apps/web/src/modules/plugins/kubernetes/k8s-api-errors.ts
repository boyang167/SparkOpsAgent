import type { ApiErrorInfo } from "./api";

type KnownK8sError = {
  message: string;
  hint?: string;
};

const KNOWN_K8S_ERRORS: Record<string, KnownK8sError> = {
  PLUGIN_CLOUDTTY_CRD_UNAVAILABLE: {
    message: "当前集群未安装 CloudTTY",
    hint:
      "CloudShell 依赖集群内的 CloudTTY Operator（CRD cloudshells.cloudshell.cloudtty.io）。" +
      "请在目标集群安装 CloudTTY 后刷新页面并重试连接。",
  },
  PLUGIN_PROMETHEUS_NOT_CONFIGURED: {
    message: "尚未配置 Prometheus",
    hint:
      "节点监控依赖 Prometheus 查询地址。请在「集群」中编辑该集群，于「Prometheus 监控」填写地址并保存后刷新。",
  },
};

const KNOWN_K8S_MESSAGE_ERRORS: Record<string, KnownK8sError> = {
  "NodeTTY Pod 就绪超时": {
    message: "临时 Pod 启动超时",
    hint: "节点可能资源不足或镜像拉取较慢。请稍后重试，或检查 kube-system 中 sparkops-nodetty 相关 Pod 事件。",
  },
};

const CRD_GROUP_HINTS: Record<string, KnownK8sError> = {
  "apps.kruise.io": {
    message: "当前集群未安装 OpenKruise",
    hint: "CloneSet、AdvancedCronJob 等资源依赖 OpenKruise。请在集群安装 Kruise Manager 后刷新页面。",
  },
  "gateway.networking.k8s.io": {
    message: "当前集群未安装 Gateway API",
    hint:
      "GatewayClass、Gateway、HTTPRoute 等资源依赖 Gateway API CRD 与 Ingress/Gateway 控制器。" +
      "请在集群安装后刷新本页；安装前可先查看 GatewayClass 是否已注册。",
  },
};

export type HumanizedApiErrorInfo = ApiErrorInfo & {
  hint?: string;
  known?: boolean;
};

export function isKnownK8sApiError(info: ApiErrorInfo | null | undefined, code?: string): boolean {
  if (!info) return false;
  const humanized = humanizeK8sApiError(info);
  if (!humanized.known) return false;
  if (code) return info.code === code;
  return true;
}

export function humanizeResourceLoadError(
  info: ApiErrorInfo,
  kind: string,
  group?: string,
): HumanizedApiErrorInfo {
  const base = humanizeK8sApiError(info);
  if (base.known) return base;
  if (info.status !== 404) return base;
  const known =
    (group ? CRD_GROUP_HINTS[group] : undefined) ??
    (group
      ? {
          message: `${kind} 资源不可用`,
          hint: `集群可能未安装 ${group} 对应的 CRD 或控制器。请确认扩展组件已部署后刷新。`,
        }
      : undefined);
  if (!known) return base;
  return {
    ...info,
    message: known.message,
    hint: known.hint,
    detail: undefined,
    known: true,
  };
}

export function isCrdUnavailableError(
  info: ApiErrorInfo | null | undefined,
  kind: string,
  group?: string,
): boolean {
  if (!info || info.status !== 404) return false;
  return humanizeResourceLoadError(info, kind, group).known === true;
}

export function humanizeK8sApiError(info: ApiErrorInfo): HumanizedApiErrorInfo {
  const known =
    (info.code ? KNOWN_K8S_ERRORS[info.code] : undefined) ??
    KNOWN_K8S_MESSAGE_ERRORS[info.message];
  if (!known) return info;
  return {
    ...info,
    message: known.message,
    hint: known.hint,
    detail: undefined,
    known: true,
  };
}
