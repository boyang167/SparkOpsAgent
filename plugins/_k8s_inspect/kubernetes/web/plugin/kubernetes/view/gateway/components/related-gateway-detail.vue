<template>
  <div v-loading="loading">
    <el-table v-if="gateways.length > 0" :data="gateways" border>
      <el-table-column type="expand">
        <template #default="scope">
          <el-descriptions :column="2" border style="margin-bottom: 12px;">
            <el-descriptions-item label="Gateway">
              {{ scope.row.metadata?.name || '-' }}
            </el-descriptions-item>
            <el-descriptions-item label="命名空间">
              {{ scope.row.metadata?.namespace || '-' }}
            </el-descriptions-item>
            <el-descriptions-item label="GatewayClass">
              {{ scope.row.spec?.gatewayClassName || '-' }}
            </el-descriptions-item>
            <el-descriptions-item label="API 版本">
              {{ scope.row.apiVersion || '-' }}
            </el-descriptions-item>
            <el-descriptions-item label="Parent Section">
              {{ scope.row._parentRef?.sectionName || '-' }}
            </el-descriptions-item>
            <el-descriptions-item label="地址">
              {{ formatAddresses(scope.row) || '-' }}
            </el-descriptions-item>
          </el-descriptions>

          <el-table v-if="scope.row.spec?.listeners?.length > 0" :data="scope.row.spec.listeners" border>
            <el-table-column label="监听器" prop="name" min-width="120" />
            <el-table-column label="协议" prop="protocol" width="100" />
            <el-table-column label="端口" prop="port" width="100" />
            <el-table-column label="域名" prop="hostname" min-width="180">
              <template #default="listenerScope">
                {{ listenerScope.row.hostname || '-' }}
              </template>
            </el-table-column>
          </el-table>
          <el-empty v-else description="暂无监听器" />

          <el-table
            v-if="scope.row.status?.conditions?.length > 0"
            :data="scope.row.status.conditions"
            border
            style="margin-top: 12px;"
          >
            <el-table-column label="类型" prop="type" width="180" />
            <el-table-column label="状态" prop="status" width="100">
              <template #default="conditionScope">
                <el-tag :type="conditionScope.row.status === 'True' ? 'success' : 'danger'" size="small">
                  {{ conditionScope.row.status }}
                </el-tag>
              </template>
            </el-table-column>
            <el-table-column label="原因" prop="reason" width="180" />
            <el-table-column label="消息" prop="message" />
          </el-table>
        </template>
      </el-table-column>
      <el-table-column label="Gateway 名称" prop="metadata.name" min-width="180" />
      <el-table-column label="命名空间" prop="metadata.namespace" width="150" />
      <el-table-column label="GatewayClass" prop="spec.gatewayClassName" min-width="180" />
      <el-table-column label="Parent Section" min-width="140">
        <template #default="scope">
          {{ scope.row._parentRef?.sectionName || '-' }}
        </template>
      </el-table-column>
      <el-table-column label="地址" min-width="220" show-overflow-tooltip>
        <template #default="scope">
          {{ formatAddresses(scope.row) || '-' }}
        </template>
      </el-table-column>
      <el-table-column label="监听器" min-width="220" show-overflow-tooltip>
        <template #default="scope">
          {{ formatListeners(scope.row) || '-' }}
        </template>
      </el-table-column>
    </el-table>
    <el-empty v-else description="暂无关联 Gateway" />
  </div>
</template>

<script>
export default {
  name: 'RelatedGatewayDetail',
}
</script>

<script setup>
import { ref, watch } from 'vue'
import { ElMessage } from 'element-plus'
import { GatewayGet } from '@/plugin/kubernetes/api/gateway/Gateway'

const props = defineProps({
  routeData: {
    type: Object,
    default: () => ({})
  }
})

const loading = ref(false)
const gateways = ref([])

const getRouteNamespace = () => props.routeData?.metadata?.namespace || ''
const getClusterId = () => {
  if (props.routeData?.cluster_id) {
    return props.routeData.cluster_id
  }
  const localClusterId = parseInt(localStorage.getItem('cluster_id'))
  return Number.isNaN(localClusterId) ? 0 : localClusterId
}

const getParentRefs = () => {
  return Array.isArray(props.routeData?.spec?.parentRefs) ? props.routeData.spec.parentRefs : []
}

const getGatewayNamespace = (parentRef) => parentRef?.namespace || getRouteNamespace()

const formatAddresses = (gateway) => {
  const specAddresses = Array.isArray(gateway?.spec?.addresses) ? gateway.spec.addresses : []
  const statusAddresses = Array.isArray(gateway?.status?.addresses) ? gateway.status.addresses : []
  const addresses = specAddresses.length > 0 ? specAddresses : statusAddresses
  return addresses
    .map(item => [item.type, item.value].filter(Boolean).join(': '))
    .filter(Boolean)
    .join(', ')
}

const formatListeners = (gateway) => {
  const listeners = Array.isArray(gateway?.spec?.listeners) ? gateway.spec.listeners : []
  return listeners
    .map(item => [item.name, item.protocol, item.port].filter(Boolean).join('/'))
    .filter(Boolean)
    .join(', ')
}

const loadRelatedGateways = async() => {
  const clusterId = getClusterId()
  const parentRefs = getParentRefs()
  if (!clusterId || parentRefs.length === 0) {
    gateways.value = []
    return
  }

  const uniqueRefs = []
  const uniqueKeys = new Set()
  parentRefs.forEach((parentRef) => {
    if (!parentRef?.name) return
    const namespace = getGatewayNamespace(parentRef)
    const key = `${namespace}/${parentRef.name}`
    if (uniqueKeys.has(key)) return
    uniqueKeys.add(key)
    uniqueRefs.push({ ...parentRef, namespace })
  })

  if (uniqueRefs.length === 0) {
    gateways.value = []
    return
  }

  loading.value = true
  try {
    const results = await Promise.all(uniqueRefs.map(async(parentRef) => {
      const res = await GatewayGet(clusterId, parentRef.namespace, parentRef.name)
      if (res.code === 0 && res.data?.items) {
        return {
          ...res.data.items,
          _parentRef: parentRef,
        }
      }
      return null
    }))
    gateways.value = results.filter(Boolean)
  } catch (error) {
    gateways.value = []
    ElMessage({
      type: 'error',
      message: '查询关联 Gateway 失败',
    })
  } finally {
    loading.value = false
  }
}

watch(
  () => props.routeData,
  () => {
    loadRelatedGateways()
  },
  { deep: true, immediate: true }
)
</script>
