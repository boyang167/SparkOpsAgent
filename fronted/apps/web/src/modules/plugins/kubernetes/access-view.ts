export const ACCESS_TABS = [
  "ServiceAccount",
  "Role",
  "RoleBinding",
  "ClusterRole",
  "ClusterRoleBinding",
  "PodSecurityPolicy",
] as const;

export type AccessTab = (typeof ACCESS_TABS)[number];

const ACCESS_API_VERSION: Record<AccessTab, string> = {
  ServiceAccount: "v1",
  Role: "rbac.authorization.k8s.io/v1",
  RoleBinding: "rbac.authorization.k8s.io/v1",
  ClusterRole: "rbac.authorization.k8s.io/v1",
  ClusterRoleBinding: "rbac.authorization.k8s.io/v1",
  PodSecurityPolicy: "policy/v1beta1",
};

type BindingSubject = {
  kind?: string;
  name?: string;
};

type RoleRef = {
  kind?: string;
  name?: string;
};

export function accessTabLabel(tab: AccessTab): string {
  if (tab === "ServiceAccount") return "Service Accounts";
  if (tab === "Role") return "Roles";
  if (tab === "RoleBinding") return "Role Bindings";
  if (tab === "ClusterRole") return "Cluster Roles";
  if (tab === "ClusterRoleBinding") return "Cluster Role Bindings";
  if (tab === "PodSecurityPolicy") return "Pod Security Policies";
  return tab;
}

export function accessTabNamespaced(tab: AccessTab): boolean {
  return tab === "ServiceAccount" || tab === "Role" || tab === "RoleBinding";
}

export const ACCESS_LIST_PREVIEW = 2;

export function accessTableColSpan(tab: AccessTab): number {
  if (tab === "ServiceAccount" || tab === "Role") return 4;
  if (tab === "RoleBinding") return 8;
  if (tab === "ClusterRole" || tab === "PodSecurityPolicy") return 3;
  if (tab === "ClusterRoleBinding") return 7;
  return 4;
}

export function formatRoleRef(roleRef?: RoleRef): string {
  if (!roleRef?.kind || !roleRef?.name) return "-";
  return `${roleRef.kind}/${roleRef.name}`;
}

export function bindingSubjectNames(subjects: BindingSubject[] | undefined, kind: string): string[] {
  if (!subjects?.length) return [];
  return subjects.filter((item) => item.kind === kind).map((item) => item.name).filter(Boolean) as string[];
}

export function formatBindingSubjects(subjects: BindingSubject[] | undefined, kind: string): string {
  const names = bindingSubjectNames(subjects, kind);
  return names.length ? names.join(", ") : "-";
}

export function emptyAccessTemplate(kind: AccessTab, namespace?: string): Record<string, unknown> {
  const tpl: Record<string, unknown> = {
    apiVersion: ACCESS_API_VERSION[kind],
    kind,
    metadata: accessTabNamespaced(kind) && namespace ? { namespace } : {},
  };
  if (kind === "Role" || kind === "ClusterRole") {
    tpl.rules = [];
  }
  if (kind === "RoleBinding") {
    tpl.roleRef = { apiGroup: "rbac.authorization.k8s.io", kind: "Role", name: "" };
    tpl.subjects = [];
  }
  if (kind === "ClusterRoleBinding") {
    tpl.roleRef = { apiGroup: "rbac.authorization.k8s.io", kind: "ClusterRole", name: "" };
    tpl.subjects = [];
  }
  return tpl;
}
