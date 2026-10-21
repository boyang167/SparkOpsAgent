<template>
  <div>
    <el-row :gutter="20" class="row-box">
      <el-col :span="24">
        <el-card class="el-card">
          <detail-basic :form="detailData" />
        </el-card>
      </el-col>
      <el-col :span="24">
        <el-tabs style="margin-top: 20px" tab-position="top" type="border-card">
          <el-tab-pane label="基本信息">
            <el-descriptions :column="2" border>
              <el-descriptions-item label="GatewayClass">
                {{ detailData.spec?.gatewayClassName || '-' }}
              </el-descriptions-item>
              <el-descriptions-item label="API 版本">
                {{ detailData.apiVersion || '-' }}
              </el-descriptions-item>
              <el-descriptions-item label="Managed By">
                {{ managedByValue || '-' }}
              </el-descriptions-item>
              <el-descriptions-item label="Owning Gateway">
                {{ owningGatewayLabelSelector || '-' }}
              </el-descriptions-item>
              <el-descriptions-item label="工作负载命名空间">
                {{ workloadNamespaceLabel }}
              </el-descriptions-item>
            </el-descriptions>
          </el-tab-pane>
          <el-tab-pane label="监听器">
            <el-table v-if="detailData.spec?.listeners?.length > 0" :data="detailData.spec.listeners" border>
              <el-table-column label="名称" prop="name" width="120" />
              <el-table-column label="协议" prop="protocol" width="100" />
              <el-table-column label="端口" prop="port" width="100" />
              <el-table-column label="域名" prop="hostname" width="200" />
              <el-table-column label="TLS 模式">
                <template #default="scope">
                  {{ scope.row.tls?.mode || '-' }}
                </template>
              </el-table-column>
              <el-table-column label="证书 Secret">
                <template #default="scope">
                  {{ scope.row.tls?.certificateRefs?.[0]?.name || '-' }}
                </template>
              </el-table-column>
              <el-table-column label="允许路由类型">
                <template #default="scope">
                  <span v-if="scope.row.allowedRoutes?.kinds?.length > 0">
                    <el-tag v-for="k in scope.row.allowedRoutes.kinds" :key="k.kind" size="small" style="margin-right: 4px;">{{ k.kind }}</el-tag>
                  </span>
                  <span v-else>-</span>
                </template>
              </el-table-column>
              <el-table-column label="允许命名空间">
                <template #default="scope">
                  <span v-if="scope.row.allowedRoutes?.namespaces?.from">
                    <span v-if="Array.isArray(scope.row.allowedRoutes.namespaces.from) && scope.row.allowedRoutes.namespaces.from.length > 0">
                      <el-tag v-for="n in scope.row.allowedRoutes.namespaces.from" :key="n" size="small" style="margin-right: 4px;">{{ n }}</el-tag>
                    </span>
                    <span v-else-if="scope.row.allowedRoutes.namespaces.from">{{ scope.row.allowedRoutes.namespaces.from }}</span>
                    <span v-else>-</span>
                  </span>
                  <span v-else>-</span>
                </template>
              </el-table-column>
            </el-table>
            <el-empty v-else description="暂无监听器" />
          </el-tab-pane>
          <el-tab-pane label="地址">
            <el-table v-if="detailAddresses.length > 0" :data="detailAddresses" border>
              <el-table-column label="类型" prop="type" width="150" />
              <el-table-column label="值" prop="value" />
            </el-table>
            <el-empty v-else description="暂无地址" />
          </el-tab-pane>
          <el-tab-pane label="关联 GatewayClass">
            <div v-loading="gatewayClassLoading">
              <el-descriptions v-if="gatewayClassData.metadata?.name" :column="2" border>
                <el-descriptions-item label="名称">
                  {{ gatewayClassData.metadata?.name || '-' }}
                </el-descriptions-item>
                <el-descriptions-item label="控制器">
                  {{ gatewayClassData.spec?.controllerName || '-' }}
                </el-descriptions-item>
                <el-descriptions-item label="API 版本">
                  {{ gatewayClassData.apiVersion || '-' }}
                </el-descriptions-item>
                <el-descriptions-item label="描述">
                  {{ gatewayClassData.spec?.description || gatewayClassData.metadata?.annotations?.description || '-' }}
                </el-descriptions-item>
              </el-descriptions>
              <el-table
                v-if="gatewayClassData.status?.conditions?.length > 0"
                :data="gatewayClassData.status.conditions"
                border
                style="margin-top: 12px;"
              >
                <el-table-column label="类型" prop="type" width="180" />
                <el-table-column label="状态" prop="status" width="100">
                  <template #default="scope">
                    <el-tag :type="scope.row.status === 'True' ? 'success' : 'danger'" size="small">{{ scope.row.status }}</el-tag>
                  </template>
                </el-table-column>
                <el-table-column label="原因" prop="reason" width="180" />
                <el-table-column label="消息" prop="message" />
              </el-table>
              <el-empty v-else-if="!gatewayClassData.metadata?.name" description="暂无关联 GatewayClass 信息" />
            </div>
          </el-tab-pane>
          <el-tab-pane label="状态">
            <div v-if="detailData.status?.conditions?.length > 0">
              <el-table :data="detailData.status.conditions" border>
                <el-table-column label="类型" prop="type" width="200" />
                <el-table-column label="状态" prop="status" width="100">
                  <template #default="scope">
                    <el-tag :type="scope.row.status === 'True' ? 'success' : 'danger'" size="small">{{ scope.row.status }}</el-tag>
                  </template>
                </el-table-column>
                <el-table-column label="原因" prop="reason" width="200" />
                <el-table-column label="消息" prop="message" />
              </el-table>
            </div>
            <el-empty v-else description="暂无状态" />
          </el-tab-pane>
          <el-tab-pane label="关联工作负载">
            <el-alert
              :title="owningGatewayLabelValue ? `按 gateway.envoyproxy.io/owning-gateway-name=${owningGatewayLabelValue} 关联 Service、Deployment 和 Pod` : '当前 Gateway 名称为空，无法按唯一标签关联 Service、Deployment 和 Pod'"
              type="success"
              :closable="false"
              style="margin-bottom: 12px;"
            />
            <div v-loading="workloadsLoading">
              <el-divider content-position="left">Service</el-divider>
              <el-table v-if="services.length > 0" :data="services" border>
                <el-table-column label="名称" prop="metadata.name" min-width="180" />
                <el-table-column label="命名空间" prop="metadata.namespace" width="150" />
                <el-table-column label="类型" prop="spec.type" width="120">
                  <template #default="scope">
                    {{ scope.row.spec?.type || 'ClusterIP' }}
                  </template>
                </el-table-column>
                <el-table-column label="状态" min-width="120">
                  <template #default="scope">
                    <el-tag :type="getServiceStatusType(scope.row)" size="small">
                      {{ getServiceStatus(scope.row) }}
                    </el-tag>
                  </template>
                </el-table-column>
                <el-table-column label="ClusterIP" min-width="160">
                  <template #default="scope">
                    {{ scope.row.spec?.clusterIP || '-' }}
                  </template>
                </el-table-column>
                <el-table-column label="端口" min-width="260" show-overflow-tooltip>
                  <template #default="scope">
                    {{ formatServicePorts(scope.row) || '-' }}
                  </template>
                </el-table-column>
                <el-table-column label="存活时间" min-width="140">
                  <template #default="scope">
                    {{ AgeFormat(scope.row.metadata?.creationTimestamp) }}
                  </template>
                </el-table-column>
              </el-table>
              <el-empty v-else description="暂无关联 Service" />

              <el-divider content-position="left">Deployment</el-divider>
              <el-table v-if="deployments.length > 0" :data="deployments" border>
                <el-table-column label="名称" prop="metadata.name" min-width="180" />
                <el-table-column label="命名空间" prop="metadata.namespace" width="150" />
                <el-table-column label="副本数" min-width="180">
                  <template #default="scope">
                    {{ scope.row.status?.readyReplicas || 0 }}/{{ scope.row.spec?.replicas || 0 }}
                  </template>
                </el-table-column>
                <el-table-column label="镜像" min-width="260" show-overflow-tooltip>
                  <template #default="scope">
                    {{ getDeploymentImages(scope.row) || '-' }}
                  </template>
                </el-table-column>
                <el-table-column label="存活时间" min-width="140">
                  <template #default="scope">
                    {{ AgeFormat(scope.row.metadata?.creationTimestamp) }}
                  </template>
                </el-table-column>
              </el-table>
              <el-empty v-else description="暂无关联 Deployment" />

              <el-divider content-position="left">Pod</el-divider>
              <el-table v-if="pods.length > 0" :data="pods" border>
                <el-table-column label="名称" prop="metadata.name" min-width="220" show-overflow-tooltip />
                <el-table-column label="命名空间" prop="metadata.namespace" width="150" />
                <el-table-column label="状态" min-width="120">
                  <template #default="scope">
                    <el-tag :type="getPodStatusType(scope.row)" size="small">
                      {{ getPodStatus(scope.row) }}
                    </el-tag>
                  </template>
                </el-table-column>
                <el-table-column label="节点" prop="spec.nodeName" min-width="180" show-overflow-tooltip />
                <el-table-column label="IP" prop="status.podIP" min-width="140" />
                <el-table-column label="存活时间" min-width="140">
                  <template #default="scope">
                    {{ AgeFormat(scope.row.metadata?.creationTimestamp) }}
                  </template>
                </el-table-column>
              </el-table>
              <el-empty v-else description="暂无关联 Pod" />
            </div>
          </el-tab-pane>
        </el-tabs>
      </el-col>
    </el-row>
  </div>
</template>

<script>
export default {
  name: 'DetailBlock',
}
</script>
<script setup>
import { computed, ref, watch } from 'vue'
import { ElMessage } from 'element-plus'
import DetailBasic from '@/plugin/kubernetes/components/detail/detail-basic.vue'
import { AgeFormat } from '@/plugin/kubernetes/utils/age'
import { ServicesList } from '@/plugin/kubernetes/api/services'
import { DeploymentsList } from '@/plugin/kubernetes/api/deployments'
import { PodsList } from '@/plugin/kubernetes/api/pods'
import { GatewayClassGet } from '@/plugin/kubernetes/api/gateway/GatewayClass'

const props = defineProps({
  data: {
    default: function() {
      return {}
    },
    type: Object
  },
})

const activeName = ref('listeners')
const detailData = ref({})
const workloadsLoading = ref(false)
const gatewayClassLoading = ref(false)
const services = ref([])
const deployments = ref([])
const pods = ref([])
const gatewayClassData = ref({})
const managedByLabelKey = 'app.kubernetes.io/managed-by'
const owningGatewayLabelKey = 'gateway.envoyproxy.io/owning-gateway-name'
const managedByValue = computed(() => detailData.value?.metadata?.labels?.[managedByLabelKey] || '-')
const owningGatewayLabelValue = computed(() => detailData.value?.metadata?.name || '')
const owningGatewayLabelSelector = computed(() => owningGatewayLabelValue.value ? `${owningGatewayLabelKey}=${owningGatewayLabelValue.value}` : '')
const workloadNamespaceLabel = computed(() => detailData.value?.metadata?.namespace || 'All Namespaces')
const detailAddresses = computed(() => {
  const specAddresses = Array.isArray(detailData.value?.spec?.addresses) ? detailData.value.spec.addresses : []
  if (specAddresses.length > 0) {
    return specAddresses
  }
  return Array.isArray(detailData.value?.status?.addresses) ? detailData.value.status.addresses : []
})
const gatewayClassName = computed(() => detailData.value?.spec?.gatewayClassName || '')

const matchOwningGatewayLabel = (item) => item?.metadata?.labels?.[owningGatewayLabelKey] === owningGatewayLabelValue.value

const getDeploymentImages = (deployment) => {
  const containers = Array.isArray(deployment?.spec?.template?.spec?.containers)
    ? deployment.spec.template.spec.containers
    : []
  return containers.map(item => item.image).filter(Boolean).join(', ')
}

const formatServicePorts = (service) => {
  const ports = Array.isArray(service?.spec?.ports) ? service.spec.ports : []
  return ports.map(item => {
    const protocol = item.protocol || 'TCP'
    const targetPort = item.targetPort ? ` -> ${item.targetPort}` : ''
    return `${item.port}/${protocol}${targetPort}`
  }).join(', ')
}

const getServiceStatus = (service) => {
  const serviceType = service?.spec?.type || 'ClusterIP'
  const loadBalancerIngress = Array.isArray(service?.status?.loadBalancer?.ingress)
    ? service.status.loadBalancer.ingress
    : []

  if (serviceType === 'ExternalName') {
    return service?.spec?.externalName ? 'Active' : 'Pending'
  }

  if (serviceType === 'LoadBalancer') {
    return loadBalancerIngress.length > 0 ? 'Active' : 'Pending'
  }

  if (service?.spec?.clusterIP && service.spec.clusterIP !== 'None') {
    return 'Active'
  }

  if (service?.spec?.clusterIP === 'None') {
    return 'Headless'
  }

  return 'Pending'
}

const getServiceStatusType = (service) => {
  const status = getServiceStatus(service)
  if (status === 'Active' || status === 'Headless') return 'success'
  return 'warning'
}

const getPodStatus = (pod) => {
  if (pod?.metadata?.deletionTimestamp) {
    return 'Deleting'
  }
  return pod?.status?.phase || '-'
}

const getPodStatusType = (pod) => {
  const status = getPodStatus(pod)
  if (status === 'Running') return 'success'
  if (status === 'Succeeded') return 'info'
  if (status === 'Deleting') return 'danger'
  return 'danger'
}

const loadRelatedWorkloads = async() => {
  const clusterId = detailData.value?.cluster_id
  if (!clusterId || !owningGatewayLabelValue.value) {
    services.value = []
    deployments.value = []
    pods.value = []
    return
  }
  workloadsLoading.value = true
  try {
    const serviceRes = await ServicesList(clusterId, 1, 1000, 'All Namespaces', '', '')
    if (serviceRes.code === 0) {
      const serviceItems = Array.isArray(serviceRes.data?.items) ? serviceRes.data.items : []
      services.value = serviceItems.filter(matchOwningGatewayLabel)
    } else {
      services.value = []
    }

    const deploymentRes = await DeploymentsList(clusterId, 1, 1000, 'All Namespaces', '')
    if (deploymentRes.code === 0) {
      const deploymentItems = Array.isArray(deploymentRes.data?.items) ? deploymentRes.data.items : []
      deployments.value = deploymentItems.filter(matchOwningGatewayLabel)
    } else {
      deployments.value = []
    }

    const podRes = await PodsList(clusterId, 1, 1000, 'All Namespaces', '', owningGatewayLabelSelector.value, '')
    if (podRes.code === 0) {
      const podItems = Array.isArray(podRes.data?.items) ? podRes.data.items : []
      pods.value = podItems.filter(matchOwningGatewayLabel)
    } else {
      pods.value = []
    }
  } catch (error) {
    services.value = []
    deployments.value = []
    pods.value = []
    ElMessage({
      type: 'error',
      message: '查询关联 Service/Deployment/Pod 失败',
    })
  } finally {
    workloadsLoading.value = false
  }
}

const loadGatewayClassDetail = async() => {
  const clusterId = detailData.value?.cluster_id
  if (!clusterId || !gatewayClassName.value) {
    gatewayClassData.value = {}
    return
  }
  gatewayClassLoading.value = true
  try {
    const res = await GatewayClassGet(clusterId, gatewayClassName.value)
    if (res.code === 0) {
      gatewayClassData.value = res.data?.items || {}
    } else {
      gatewayClassData.value = {}
    }
  } catch (error) {
    gatewayClassData.value = {}
    ElMessage({
      type: 'error',
      message: '查询关联 GatewayClass 失败',
    })
  } finally {
    gatewayClassLoading.value = false
  }
}

const detailInitData = () => {
  activeName.value = 'listeners'
  detailData.value = props.data || {}
  loadGatewayClassDetail()
  loadRelatedWorkloads()
}

detailInitData()

watch(
  () => props.data,
  () => {
    detailInitData()
  },
  { deep: true }
)
</script>
<style scoped>

</style>
