INSERT INTO plugin.plugins (
    id, version, display_name, manifest_yaml, package_digest, installed_files,
    status, pr_url, created_by, created_at, updated_at
) VALUES (
    'kubernetes',
    '0.1.0',
    'Kubernetes',
    $manifest$id: kubernetes
version: "0.1.0"
display_name: Kubernetes
route_prefix: /api/v1/plugins/kubernetes
entitlement: plugin:kubernetes
menus:
  - title: Kubernetes管理
    path: /plugins/kubernetes
    component: plugins/kubernetes/page
  - title: 集群管理
    path: /plugins/kubernetes/clusters
    component: plugins/kubernetes/clusters/page
    parent: /plugins/kubernetes
  - title: 节点管理
    path: /plugins/kubernetes/nodes
    component: plugins/kubernetes/nodes/page
    parent: /plugins/kubernetes
  - title: 工作负载
    path: /plugins/kubernetes/workloads
    component: plugins/kubernetes/workloads/page
    parent: /plugins/kubernetes
  - title: OpenKruise
    path: /plugins/kubernetes/openkruise
    component: plugins/kubernetes/openkruise/page
    parent: /plugins/kubernetes
  - title: Kruise Rollouts
    path: /plugins/kubernetes/kruise-rollouts
    component: plugins/kubernetes/kruise-rollouts/page
    parent: /plugins/kubernetes
  - title: KubeBlocks
    path: /plugins/kubernetes/kubeblocks
    component: plugins/kubernetes/kubeblocks/page
    parent: /plugins/kubernetes
  - title: KubeVela
    path: /plugins/kubernetes/kubevela
    component: plugins/kubernetes/kubevela/page
    parent: /plugins/kubernetes
  - title: 命名空间
    path: /plugins/kubernetes/namespaces
    component: plugins/kubernetes/namespaces/page
    parent: /plugins/kubernetes
  - title: 网络管理
    path: /plugins/kubernetes/networks
    component: plugins/kubernetes/networks/page
    parent: /plugins/kubernetes
  - title: Gateway
    path: /plugins/kubernetes/gateway
    component: plugins/kubernetes/gateway/page
    parent: /plugins/kubernetes
  - title: 配置管理
    path: /plugins/kubernetes/configs
    component: plugins/kubernetes/configs/page
    parent: /plugins/kubernetes
  - title: 存储管理
    path: /plugins/kubernetes/storages
    component: plugins/kubernetes/storages/page
    parent: /plugins/kubernetes
  - title: 访问控制
    path: /plugins/kubernetes/access-controls
    component: plugins/kubernetes/access-controls/page
    parent: /plugins/kubernetes
  - title: 终端审计
    path: /plugins/kubernetes/records
    component: plugins/kubernetes/records/page
    parent: /plugins/kubernetes
  - title: 应用诊断
    path: /plugins/kubernetes/diagnosis
    component: plugins/kubernetes/diagnosis/page
    parent: /plugins/kubernetes
  - title: Velero备份
    path: /plugins/kubernetes/backup
    component: plugins/kubernetes/backup/page
    parent: /plugins/kubernetes
  - title: AI Chat
    path: /plugins/kubernetes/chat
    component: plugins/kubernetes/chat/page
    parent: /plugins/kubernetes
  - title: CloudTTY
    path: /plugins/kubernetes/cloudtty
    component: plugins/kubernetes/cloudtty/page
    parent: /plugins/kubernetes
  - title: Velaux
    path: /plugins/kubernetes/velaux
    component: plugins/kubernetes/velaux/page
    parent: /plugins/kubernetes
apis:
  - method: GET
    path: /api/v1/plugins/kubernetes/ready
  - method: GET
    path: /api/v1/plugins/kubernetes/clusters
  - method: GET
    path: /api/v1/plugins/kubernetes/clusters/:clusterId/kubeconfig
  - method: GET
    path: /api/v1/plugins/kubernetes/clusters/:clusterId/users
  - method: POST
    path: /api/v1/plugins/kubernetes/clusters/:clusterId/users
  - method: PUT
    path: /api/v1/plugins/kubernetes/clusters/:clusterId/users/:memberId
  - method: DELETE
    path: /api/v1/plugins/kubernetes/clusters/:clusterId/users/:memberId
  - method: POST
    path: /api/v1/plugins/kubernetes/clusters/:clusterId/users/:memberId/kubeconfig
  - method: PUT
    path: /api/v1/plugins/kubernetes/clusters/:clusterId/settings
  - method: ANY
    path: /api/v1/plugins/kubernetes/clusters/:clusterId/proxy/*
  - method: POST
    path: /api/v1/plugins/kubernetes/clusters/:clusterId/metrics
    read_only: true
  - method: GET
    path: /api/v1/plugins/kubernetes/clusters/:clusterId/namespaces/:namespace/pods/:pod/logs
  - method: GET
    path: /api/v1/plugins/kubernetes/clusters/:clusterId/namespaces/:namespace/pods/:pod/exec
  - method: POST
    path: /api/v1/plugins/kubernetes/clusters/:clusterId/namespaces/:namespace/pods/:pod/terminal-ticket
  - method: GET
    path: /api/v1/plugins/kubernetes/clusters/:clusterId/namespaces/:namespace/pods/:pod/files
  - method: GET
    path: /api/v1/plugins/kubernetes/clusters/:clusterId/namespaces/:namespace/pods/:pod/files/download
  - method: POST
    path: /api/v1/plugins/kubernetes/clusters/:clusterId/namespaces/:namespace/pods/:pod/files
  - method: DELETE
    path: /api/v1/plugins/kubernetes/clusters/:clusterId/namespaces/:namespace/pods/:pod/files
  - method: GET
    path: /api/v1/plugins/kubernetes/records
  - method: GET
    path: /api/v1/plugins/kubernetes/records/:recordId
  - method: DELETE
    path: /api/v1/plugins/kubernetes/records/:recordId
  - method: POST
    path: /api/v1/plugins/kubernetes/clusters/:clusterId/cloudtty
  - method: POST
    path: /api/v1/plugins/kubernetes/clusters/:clusterId/nodetty
  - method: POST
    path: /api/v1/plugins/kubernetes/clusters/:clusterId/diagnosis
  - method: POST
    path: /api/v1/plugins/kubernetes/clusters/:clusterId/diagnosis/install
  - method: DELETE
    path: /api/v1/plugins/kubernetes/clusters/:clusterId/diagnosis/uninstall
  - method: POST
    path: /api/v1/plugins/kubernetes/clusters/:clusterId/diagnosis/by-pod
  - method: POST
    path: /api/v1/plugins/kubernetes/clusters/:clusterId/diagnosis/jps
  - method: POST
    path: /api/v1/plugins/kubernetes/clusters/:clusterId/diagnosis/proxy
  - method: GET
    path: /api/v1/plugins/kubernetes/diagnosis/:id/ws
  - method: DELETE
    path: /api/v1/plugins/kubernetes/diagnosis/:id
  - method: ANY
    path: /api/v1/plugins/kubernetes/clusters/:clusterId/velaux/*
  - method: POST
    path: /api/v1/plugins/kubernetes/clusters/:clusterId/chat/query
frontend:
  pages:
    - app/(platform)/plugins/kubernetes/page.tsx
    - app/(platform)/plugins/kubernetes/layout.tsx
    - app/(platform)/plugins/kubernetes/clusters/page.tsx
    - app/(platform)/plugins/kubernetes/nodes/page.tsx
    - app/(platform)/plugins/kubernetes/workloads/page.tsx
    - app/(platform)/plugins/kubernetes/openkruise/page.tsx
    - app/(platform)/plugins/kubernetes/kruise-rollouts/page.tsx
    - app/(platform)/plugins/kubernetes/kubeblocks/page.tsx
    - app/(platform)/plugins/kubernetes/kubevela/page.tsx
    - app/(platform)/plugins/kubernetes/namespaces/page.tsx
    - app/(platform)/plugins/kubernetes/networks/page.tsx
    - app/(platform)/plugins/kubernetes/gateway/page.tsx
    - app/(platform)/plugins/kubernetes/configs/page.tsx
    - app/(platform)/plugins/kubernetes/storages/page.tsx
    - app/(platform)/plugins/kubernetes/access-controls/page.tsx
    - app/(platform)/plugins/kubernetes/backup/page.tsx
    - app/(platform)/plugins/kubernetes/records/page.tsx
    - app/(platform)/plugins/kubernetes/diagnosis/page.tsx
    - app/(platform)/plugins/kubernetes/chat/page.tsx
    - app/(platform)/plugins/kubernetes/cloudtty/page.tsx
    - app/(platform)/plugins/kubernetes/velaux/page.tsx
assets:
  - backend/internal/plugins/kubernetes/resource/arthas-bin.tar
$manifest$,
    '',
    '["backend/internal/plugins/kubernetes/plugin.go","backend/internal/plugins/kubernetes/plugin.yaml","backend/internal/plugins/kubernetes/chat/query.go","backend/internal/plugins/kubernetes/diagnosis/arthas.go","backend/internal/plugins/kubernetes/files/handler.go","backend/internal/plugins/kubernetes/http/clusters.go","backend/internal/plugins/kubernetes/http/cluster_users.go","backend/internal/plugins/kubernetes/http/kubeconfig.go","backend/internal/plugins/kubernetes/http/metrics.go","backend/internal/plugins/kubernetes/http/router.go","backend/internal/plugins/kubernetes/http/settings.go","backend/internal/plugins/kubernetes/http/settings_store.go","backend/internal/plugins/kubernetes/http/write_guard.go","backend/internal/plugins/kubernetes/metrics/query.go","backend/internal/plugins/kubernetes/proxy/aggregate.go","backend/internal/plugins/kubernetes/proxy/auth.go","backend/internal/plugins/kubernetes/proxy/discovery_cache.go","backend/internal/plugins/kubernetes/proxy/handler.go","backend/internal/plugins/kubernetes/proxy/path.go","backend/internal/plugins/kubernetes/proxy/redaction.go","backend/internal/plugins/kubernetes/proxy/rewrite.go","backend/internal/plugins/kubernetes/proxy/transport.go","backend/internal/plugins/kubernetes/rbac/apply.go","backend/internal/plugins/kubernetes/rbac/names.go","backend/internal/plugins/kubernetes/records/handler.go","backend/internal/plugins/kubernetes/resource/arthas-bin.tar","backend/internal/plugins/kubernetes/resource/assets.go","backend/internal/plugins/kubernetes/schema/migrate.go","backend/internal/plugins/kubernetes/schema/models.go","backend/internal/plugins/kubernetes/stream/exec.go","backend/internal/plugins/kubernetes/stream/logs.go","backend/internal/plugins/kubernetes/terminal/handler.go","backend/internal/plugins/kubernetes/terminal/protocol.go","backend/internal/plugins/kubernetes/terminal/session.go","backend/internal/plugins/kubernetes/terminal/ticket.go","backend/internal/plugins/kubernetes/tty/cloudtty.go","backend/internal/plugins/kubernetes/tty/nodetty.go","backend/internal/plugins/kubernetes/velaux/proxy.go","fronted/apps/web/app/(platform)/plugins/kubernetes/page.tsx","fronted/apps/web/app/(platform)/plugins/kubernetes/layout.tsx","fronted/apps/web/app/(platform)/plugins/kubernetes/clusters/page.tsx","fronted/apps/web/app/(platform)/plugins/kubernetes/nodes/page.tsx","fronted/apps/web/app/(platform)/plugins/kubernetes/workloads/page.tsx","fronted/apps/web/app/(platform)/plugins/kubernetes/openkruise/page.tsx","fronted/apps/web/app/(platform)/plugins/kubernetes/kruise-rollouts/page.tsx","fronted/apps/web/app/(platform)/plugins/kubernetes/kubeblocks/page.tsx","fronted/apps/web/app/(platform)/plugins/kubernetes/kubevela/page.tsx","fronted/apps/web/app/(platform)/plugins/kubernetes/namespaces/page.tsx","fronted/apps/web/app/(platform)/plugins/kubernetes/networks/page.tsx","fronted/apps/web/app/(platform)/plugins/kubernetes/gateway/page.tsx","fronted/apps/web/app/(platform)/plugins/kubernetes/configs/page.tsx","fronted/apps/web/app/(platform)/plugins/kubernetes/storages/page.tsx","fronted/apps/web/app/(platform)/plugins/kubernetes/access-controls/page.tsx","fronted/apps/web/app/(platform)/plugins/kubernetes/backup/page.tsx","fronted/apps/web/app/(platform)/plugins/kubernetes/records/page.tsx","fronted/apps/web/app/(platform)/plugins/kubernetes/diagnosis/page.tsx","fronted/apps/web/app/(platform)/plugins/kubernetes/chat/page.tsx","fronted/apps/web/app/(platform)/plugins/kubernetes/cloudtty/page.tsx","fronted/apps/web/app/(platform)/plugins/kubernetes/velaux/page.tsx","fronted/apps/web/app/api/bff/v1/plugins/kubernetes/[...path]/route.ts","fronted/apps/web/src/modules/plugins/kubernetes/ApiErrorBanner.tsx","fronted/apps/web/src/modules/plugins/kubernetes/ClusterPicker.tsx","fronted/apps/web/src/modules/plugins/kubernetes/ClusterCreateForm.tsx","fronted/apps/web/src/modules/plugins/kubernetes/cluster-settings-form.tsx","fronted/apps/web/src/modules/plugins/kubernetes/cluster-status.ts","fronted/apps/web/src/modules/plugins/kubernetes/cluster-types.ts","fronted/apps/web/src/modules/plugins/kubernetes/node-view.ts","fronted/apps/web/src/modules/plugins/kubernetes/pod-terminal.ts","fronted/apps/web/src/modules/plugins/kubernetes/KubernetesConsole.tsx","fronted/apps/web/src/modules/plugins/kubernetes/OperationsPanel.tsx","fronted/apps/web/src/modules/plugins/kubernetes/ResourceMenuPage.tsx","fronted/apps/web/src/modules/plugins/kubernetes/ResourceTable.tsx","fronted/apps/web/src/modules/plugins/kubernetes/YamlEditor.tsx","fronted/apps/web/src/modules/plugins/kubernetes/api.ts","fronted/apps/web/src/modules/plugins/kubernetes/catalog.ts","fronted/apps/web/src/modules/plugins/kubernetes/page-server.tsx","fronted/apps/web/src/modules/plugins/kubernetes/operations.ts","fronted/apps/web/src/modules/plugins/kubernetes/resource-url.ts","fronted/apps/web/src/modules/plugins/kubernetes/sse.ts","fronted/apps/web/src/modules/plugins/kubernetes/velaux-frame.ts","fronted/apps/web/src/modules/plugins/kubernetes/yaml-document.ts","fronted/apps/web/src/modules/plugins/kubernetes/specialized/ChatPage.tsx","fronted/apps/web/src/modules/plugins/kubernetes/specialized/CloudTTYPage.tsx","fronted/apps/web/src/modules/plugins/kubernetes/specialized/ClusterAssociateDialog.tsx","fronted/apps/web/src/modules/plugins/kubernetes/specialized/ClusterAuthorizeDialog.tsx","fronted/apps/web/src/modules/plugins/kubernetes/specialized/ClusterAuthorizeRoles.tsx","fronted/apps/web/src/modules/plugins/kubernetes/specialized/cluster-authorize-roles.ts","fronted/apps/web/src/modules/plugins/kubernetes/specialized/ClusterAuthorizeUsers.tsx","fronted/apps/web/src/modules/plugins/kubernetes/specialized/ClustersPanel.tsx","fronted/apps/web/src/modules/plugins/kubernetes/specialized/DiagnosisPanel.tsx","fronted/apps/web/src/modules/plugins/kubernetes/specialized/FormParts.tsx","fronted/apps/web/src/modules/plugins/kubernetes/specialized/NodeDetailView.tsx","fronted/apps/web/src/modules/plugins/kubernetes/specialized/NodesPanel.tsx","fronted/apps/web/src/modules/plugins/kubernetes/specialized/OverviewPanel.tsx","fronted/apps/web/src/modules/plugins/kubernetes/specialized/PodTerminal.tsx","fronted/apps/web/src/modules/plugins/kubernetes/specialized/RecordsPage.tsx","fronted/apps/web/src/modules/plugins/kubernetes/specialized/VelauxPage.tsx"]'::jsonb,
    'deployed',
    '',
    '00000000-0000-0000-0000-000000000000',
    now(),
    now()
)
ON CONFLICT (id) DO NOTHING;
