<template>
  <div>
    <el-card class="el-card">
      <el-form ref="FormBlock" :model="formData" :rules="rules" label-width="120px">
        <el-row :gutter="24">
          <el-col :span="12">
            <el-form-item label="名称" prop="metadata.name">
              <el-input v-model="formData.metadata.name" placeholder="请输入名称" :disabled="readOnly || isEdit" />
            </el-form-item>
          </el-col>
          <el-col :span="12">
            <el-form-item label="命名空间" prop="metadata.namespace">
              <el-input v-if="readOnly || isEdit" :model-value="formData.metadata.namespace" disabled style="width: 100%" />
              <NsSelect v-else :cluster="cluster_id" :namespace.sync="formData.metadata.namespace" style="width: 100%" @enter="enterNamespace" />
            </el-form-item>
          </el-col>
        </el-row>

        <el-divider content-position="left">Gateway 引用</el-divider>
        <div v-for="(parentRef, idx) in formData.spec.parentRefs" :key="idx" class="section-card">
          <div class="section-header">
            <span>Gateway 引用 {{ idx + 1 }}</span>
            <el-button v-if="!readOnly" type="danger" size="small" link @click="removeParentRef(idx)">删除</el-button>
          </div>
          <el-row :gutter="16">
            <el-col :span="8">
              <el-form-item label-width="0">
                <div class="field-label required">Gateway 名称</div>
                <el-select
                  v-model="parentRef._gatewayKey"
                  :disabled="readOnly"
                  :loading="gatewaysLoading"
                  filterable
                  clearable
                  style="width: 100%;"
                  placeholder="请选择 Gateway"
                  @change="handleParentGatewayChange(parentRef)"
                >
                  <el-option
                    v-for="gateway in getAvailableGateways(parentRef)"
                    :key="getGatewayKey(gateway)"
                    :label="formatGatewayLabel(gateway)"
                    :value="getGatewayKey(gateway)"
                  />
                </el-select>
              </el-form-item>
            </el-col>
            <el-col :span="8">
              <el-form-item label-width="0">
                <div class="field-label">Namespace</div>
                <el-input v-model="parentRef.namespace" placeholder="默认同当前命名空间" disabled />
              </el-form-item>
            </el-col>
            <el-col :span="8">
              <el-form-item label-width="0">
                <div class="field-label">Section Name</div>
                <el-input v-model="parentRef.sectionName" placeholder="监听器名称（可选）" :disabled="readOnly" />
              </el-form-item>
            </el-col>
          </el-row>
        </div>
        <el-button v-if="!readOnly" size="small" type="primary" plain @click="addParentRef">添加 Gateway 引用</el-button>

        <el-divider content-position="left">Hostnames</el-divider>
        <el-form-item>
          <el-input
            v-model="hostnamesInput"
            type="textarea"
            :rows="2"
            placeholder="一行一个，支持通配符，例如 *.example.com"
            :disabled="readOnly"
            @blur="syncHostnames"
          />
          <div style="font-size: 12px; color: var(--el-text-color-secondary); margin-top: 4px;">
            多个 hostname 换行填写，留空表示匹配所有
          </div>
        </el-form-item>

        <el-divider content-position="left">路由规则</el-divider>
        <div v-for="(rule, idx) in formData.spec.rules" :key="idx" class="section-card">
          <div class="section-header">
            <span>规则 {{ idx + 1 }}</span>
            <el-button v-if="!readOnly" type="danger" size="small" link @click="removeRule(idx)">删除</el-button>
          </div>

          <div class="sub-section-title">后端服务（backendRefs）</div>
          <div v-for="(backendRef, ridx) in rule.backendRefs" :key="ridx" style="margin-bottom: 12px;">
            <el-row :gutter="12">
              <el-col :span="5">
                <div class="field-label required">服务名称</div>
                <el-form-item label-width="0" style="margin-bottom: 8px;">
                  <el-select
                    v-model="backendRef.name"
                    :disabled="readOnly"
                    :loading="servicesLoading"
                    filterable
                    clearable
                    style="width: 100%;"
                    placeholder="请选择当前命名空间的 Service"
                    @change="handleBackendServiceChange(backendRef)"
                  >
                    <el-option
                      v-for="service in services"
                      :key="service.metadata.name"
                      :label="service.metadata.name"
                      :value="service.metadata.name"
                    />
                  </el-select>
                </el-form-item>
              </el-col>
              <el-col :span="4">
                <div class="field-label">命名空间</div>
                <el-form-item label-width="0" style="margin-bottom: 8px;">
                  <el-input v-model="backendRef.namespace" placeholder="随顶部命名空间自动填充" disabled />
                </el-form-item>
              </el-col>
              <el-col :span="3">
                <div class="field-label required">端口</div>
                <el-form-item label-width="0" style="margin-bottom: 8px;">
                  <el-select
                    v-model="backendRef.port"
                    :disabled="readOnly"
                    :loading="servicesLoading"
                    :clearable="!readOnly"
                    style="width: 100%;"
                    placeholder="从所选 Service 端口中选择"
                    @change="normalizeBackendNumberField(backendRef, 'port')"
                  >
                    <el-option
                      v-for="port in getServicePorts(backendRef.name)"
                      :key="`${port.name || 'port'}-${port.port}-${port.protocol || ''}`"
                      :label="formatServicePortLabel(port)"
                      :value="String(port.port)"
                    />
                  </el-select>
                </el-form-item>
              </el-col>
              <el-col :span="3">
                <div class="field-label">权重</div>
                <el-form-item label-width="0" style="margin-bottom: 8px;">
                  <el-input
                    v-model="backendRef.weight"
                    placeholder="默认 1"
                    :disabled="readOnly"
                    inputmode="numeric"
                    @input="sanitizeBackendNumberField(backendRef, 'weight')"
                    @blur="normalizeBackendNumberField(backendRef, 'weight')"
                  />
                </el-form-item>
              </el-col>
              <el-col :span="4">
                <div class="field-label">Kind</div>
                <el-form-item label-width="0" style="margin-bottom: 8px;">
                  <el-select v-model="backendRef.kind" :disabled="readOnly" style="width: 100%;" placeholder="默认 Service">
                    <el-option v-for="item in backendKindOptions" :key="item" :label="item" :value="item" />
                  </el-select>
                </el-form-item>
              </el-col>
              <el-col :span="3">
                <div class="field-label">Group</div>
                <el-form-item label-width="0" style="margin-bottom: 8px;">
                  <el-input v-model="backendRef.group" placeholder="默认 core group" :disabled="readOnly" />
                </el-form-item>
              </el-col>
              <el-col :span="2" style="display: flex; align-items: flex-end; padding-bottom: 8px;">
                <el-button v-if="!readOnly" type="danger" size="small" link @click="rule.backendRefs.splice(ridx, 1)">删除</el-button>
              </el-col>
            </el-row>
          </div>
          <el-button v-if="!readOnly" size="small" plain @click="addBackendRef(rule)">添加后端服务</el-button>

          <div class="sub-section-title" style="margin-top: 16px;">匹配规则（matches）</div>
          <div v-for="(match, midx) in rule.matches" :key="midx" style="margin-bottom: 12px;">
            <el-row :gutter="12">
              <el-col :span="8">
                <div class="field-label">gRPC Service</div>
                <el-form-item label-width="0" style="margin-bottom: 8px;">
                  <el-input v-model="match.grpcService" placeholder="例如 helloworld.Greeter" :disabled="readOnly" />
                </el-form-item>
              </el-col>
              <el-col :span="8">
                <div class="field-label">gRPC Method</div>
                <el-form-item label-width="0" style="margin-bottom: 8px;">
                  <el-input v-model="match.grpcMethod" placeholder="例如 SayHello" :disabled="readOnly" />
                </el-form-item>
              </el-col>
              <el-col :span="6">
                <div class="field-label">类型</div>
                <el-form-item label-width="0" style="margin-bottom: 8px;">
                  <el-select v-model="match.type" :disabled="readOnly" style="width: 100%;" clearable placeholder="默认精确匹配">
                    <el-option label="Exact" value="Exact" />
                    <el-option label="RegularExpression" value="RegularExpression" />
                  </el-select>
                </el-form-item>
              </el-col>
              <el-col :span="2" style="display: flex; align-items: flex-end; padding-bottom: 8px;">
                <el-button v-if="!readOnly" type="danger" size="small" link @click="rule.matches.splice(midx, 1)">删除</el-button>
              </el-col>
            </el-row>
            <el-row :gutter="12">
              <el-col :span="24">
                <div class="field-label">Header 匹配</div>
                <el-form-item label-width="0" style="margin-bottom: 8px;">
                  <el-input
                    v-model="match.headersText"
                    type="textarea"
                    :rows="2"
                    placeholder="一行一个，格式 key=value"
                    :disabled="readOnly"
                  />
                </el-form-item>
              </el-col>
            </el-row>
          </div>
          <el-button v-if="!readOnly" size="small" plain @click="addMatch(rule)">添加匹配规则</el-button>
        </div>
        <el-button v-if="!readOnly" size="small" type="primary" plain @click="addRule">添加路由规则</el-button>
      </el-form>
      <div style="float: right; margin-top: 10px">
        <el-button size="small" @click="closeDialog">取 消</el-button>
        <el-button size="small" type="primary" @click="enterDialog">确 定</el-button>
      </div>
    </el-card>
  </div>
</template>

<script>
export default {
  name: 'GRPCRouteForm',
}
</script>

<script setup>
import { ref, watch } from 'vue'
import { ElMessage } from 'element-plus'
import { GatewayList } from '@/plugin/kubernetes/api/gateway/Gateway'
import { ServicesList } from '@/plugin/kubernetes/api/services'
import NsSelect from '@/plugin/kubernetes/components/namespace/select-ns.vue'

const emit = defineEmits(['close', 'enter'])
const props = defineProps({
  form: {
    type: Object,
    default: () => ({})
  },
  readonly: {
    type: Boolean,
    default: false
  },
  isEdit: {
    type: Boolean,
    default: false
  },
  cluster: {
    type: Number,
    default: 0
  }
})

const rules = ref({
  'metadata.name': [{ required: true, message: '请输入名称', trigger: 'blur' }],
  'metadata.namespace': [{ required: true, message: '请输入命名空间', trigger: 'blur' }],
})

const formData = ref({})
const readOnly = ref(false)
const isEdit = ref(false)
const cluster_id = ref(0)
const hostnamesInput = ref('')
const services = ref([])
const servicesLoading = ref(false)
const gateways = ref([])
const gatewaysLoading = ref(false)

const syncBackendRefNamespaces = (namespace) => {
  const routeRules = Array.isArray(formData.value?.spec?.rules) ? formData.value.spec.rules : []
  routeRules.forEach(rule => {
    const backendRefs = Array.isArray(rule?.backendRefs) ? rule.backendRefs : []
    backendRefs.forEach(backendRef => {
      backendRef.namespace = namespace || ''
    })
  })
}

const syncParentRefGatewayKeys = (namespace) => {
  const parentRefs = Array.isArray(formData.value?.spec?.parentRefs) ? formData.value.spec.parentRefs : []
  parentRefs.forEach(parentRef => {
    parentRef._gatewayKey = `${parentRef?.namespace || namespace || ''}/${parentRef?.name || ''}`
  })
}

const enterNamespace = (val) => {
  if (formData.value.metadata) {
    formData.value.metadata.namespace = val
  }
  syncBackendRefNamespaces(val)
  syncParentRefGatewayKeys(val)
}

const loadServices = async(namespace) => {
  if (!cluster_id.value || !namespace) {
    services.value = []
    return
  }
  servicesLoading.value = true
  try {
    const res = await ServicesList(cluster_id.value, 1, 1000, namespace, '', '')
    if (res.code === 0) {
      services.value = Array.isArray(res.data?.items) ? res.data.items : []
    } else {
      services.value = []
    }
  } finally {
    servicesLoading.value = false
  }
}

const loadGateways = async() => {
  if (!cluster_id.value) {
    gateways.value = []
    return
  }
  gatewaysLoading.value = true
  try {
    const res = await GatewayList(cluster_id.value, 1, 1000, '', '')
    if (res.code === 0) {
      gateways.value = Array.isArray(res.data?.items) ? res.data.items : []
    } else {
      gateways.value = []
    }
  } finally {
    gatewaysLoading.value = false
  }
}

const getGatewayKey = (gateway) => `${gateway?.metadata?.namespace || ''}/${gateway?.metadata?.name || ''}`

const formatGatewayLabel = (gateway) => `${gateway?.metadata?.name || ''} (${gateway?.metadata?.namespace || '-'})`

const isCrossNamespaceGatewayAllowed = (gateway) => {
  const listeners = Array.isArray(gateway?.spec?.listeners) ? gateway.spec.listeners : []
  return listeners.some(listener => listener?.allowedRoutes?.namespaces?.from === 'All')
}

const getAvailableGateways = (parentRef) => {
  const currentNamespace = formData.value?.metadata?.namespace || ''
  const selectedNamespace = parentRef?.namespace || currentNamespace
  const selectedName = parentRef?.name || ''
  return gateways.value.filter(gateway => {
    const gatewayNamespace = gateway?.metadata?.namespace || ''
    const gatewayName = gateway?.metadata?.name || ''
    const isSelected = gatewayNamespace === selectedNamespace && gatewayName === selectedName
    if (isSelected) {
      return true
    }
    if (gatewayNamespace === currentNamespace) {
      return true
    }
    return isCrossNamespaceGatewayAllowed(gateway)
  })
}

const handleParentGatewayChange = (parentRef) => {
  const selectedGateway = gateways.value.find(item => getGatewayKey(item) === parentRef?._gatewayKey)
  if (!selectedGateway) {
    parentRef.name = ''
    parentRef.namespace = ''
    return
  }
  const currentNamespace = formData.value?.metadata?.namespace || ''
  parentRef.name = selectedGateway.metadata?.name || ''
  parentRef.namespace = selectedGateway.metadata?.namespace === currentNamespace
    ? ''
    : (selectedGateway.metadata?.namespace || '')
}

const getServicePorts = (serviceName) => {
  if (!serviceName) {
    return []
  }
  const service = services.value.find(item => item?.metadata?.name === serviceName)
  return Array.isArray(service?.spec?.ports) ? service.spec.ports : []
}

const formatServicePortLabel = (port) => {
  if (!port) {
    return ''
  }
  const parts = [String(port.port)]
  if (port.name) {
    parts.push(port.name)
  }
  if (port.protocol) {
    parts.push(port.protocol)
  }
  return parts.join(' / ')
}

const handleBackendServiceChange = (backendRef) => {
  const servicePorts = getServicePorts(backendRef?.name)
  if (servicePorts.length === 1) {
    backendRef.port = String(servicePorts[0].port)
    return
  }
  const hasCurrentPort = servicePorts.some(item => String(item.port) === String(backendRef?.port || ''))
  if (!hasCurrentPort) {
    backendRef.port = ''
  }
}

const BACKEND_NUMBER_FIELDS = {
  port: { min: 1, max: 65535, defaultValue: '' },
  weight: { min: 0, max: Number.MAX_SAFE_INTEGER, defaultValue: '1' },
}
const backendKindOptions = ['Service']

const formatEditableNumber = (value) => {
  if (value === undefined || value === null || value === '') {
    return ''
  }
  return String(value)
}

const clampNumber = (value, min, max) => {
  const parsed = Number(value)
  if (Number.isNaN(parsed)) {
    return null
  }
  const integer = Math.trunc(parsed)
  return Math.min(max, Math.max(min, integer))
}

const parseNameValueText = (value) => {
  if (!value) {
    return []
  }
  return value
    .split('\n')
    .map(line => line.trim())
    .filter(line => line)
    .map(line => {
      const splitIndex = line.indexOf('=')
      if (splitIndex === -1) {
        return { name: line, value: '' }
      }
      return {
        name: line.slice(0, splitIndex).trim(),
        value: line.slice(splitIndex + 1).trim(),
      }
    })
    .filter(item => item.name)
}

const stringifyNameValueList = (value) => {
  if (!Array.isArray(value) || value.length === 0) {
    return ''
  }
  return value
    .map(item => `${item?.name || ''}${item?.value !== undefined ? `=${item.value || ''}` : ''}`)
    .join('\n')
}

const createBackendRef = (namespace = '') => ({
  group: '',
  kind: 'Service',
  name: '',
  namespace,
  port: BACKEND_NUMBER_FIELDS.port.defaultValue,
  weight: BACKEND_NUMBER_FIELDS.weight.defaultValue,
})
const createMatch = () => ({
  grpcService: '',
  grpcMethod: '',
  type: '',
  headersText: '',
})
const createRule = () => ({ backendRefs: [createBackendRef(formData.value?.metadata?.namespace || '')], matches: [createMatch()] })
const createParentRef = () => ({ name: '', namespace: '', sectionName: '', _gatewayKey: '' })

const toEditableBackendRef = (backendRef = {}, namespace = '') => ({
  group: backendRef.group ?? '',
  kind: backendRef.kind ?? 'Service',
  name: backendRef.name ?? '',
  namespace: backendRef.namespace ?? namespace,
  port: formatEditableNumber(backendRef.port),
  weight: formatEditableNumber(backendRef.weight),
})

const toEditableMatch = (match = {}) => ({
  grpcService: match?.method?.service || '',
  grpcMethod: match?.method?.method || '',
  type: match?.method?.type || '',
  headersText: stringifyNameValueList(match?.headers),
})

const normalizeParentRefs = () => {
  const parentRefs = Array.isArray(formData.value.spec.parentRefs) ? formData.value.spec.parentRefs : []
  const normalized = parentRefs
    .map(item => ({
      name: item?.name?.trim?.() || '',
      namespace: item?.namespace?.trim?.() || '',
      sectionName: item?.sectionName?.trim?.() || '',
    }))
    .filter(item => item.name)
    .map(item => {
      const result = { name: item.name }
      if (item.namespace) {
        result.namespace = item.namespace
      }
      if (item.sectionName) {
        result.sectionName = item.sectionName
      }
      return result
    })

  if (normalized.length > 0) {
    formData.value.spec.parentRefs = normalized
  } else {
    delete formData.value.spec.parentRefs
  }
}

const normalizeBackendRef = (backendRef) => {
  const normalized = {}
  if (backendRef.group) {
    normalized.group = backendRef.group.trim()
  }
  if (backendRef.kind) {
    normalized.kind = backendRef.kind.trim()
  }
  if (backendRef.name) {
    normalized.name = backendRef.name.trim()
  }
  if (backendRef.namespace) {
    normalized.namespace = backendRef.namespace.trim()
  }

  const port = clampNumber(backendRef.port, BACKEND_NUMBER_FIELDS.port.min, BACKEND_NUMBER_FIELDS.port.max)
  if (port !== null) {
    normalized.port = port
  }

  const weight = clampNumber(backendRef.weight, BACKEND_NUMBER_FIELDS.weight.min, BACKEND_NUMBER_FIELDS.weight.max)
  if (weight !== null) {
    normalized.weight = weight
  }

  return normalized
}

const normalizeMatch = (match) => {
  const normalized = {}
  const method = {}
  const grpcService = match?.grpcService?.trim?.() || ''
  const grpcMethod = match?.grpcMethod?.trim?.() || ''
  const type = match?.type?.trim?.() || ''
  if (grpcService) {
    method.service = grpcService
  }
  if (grpcMethod) {
    method.method = grpcMethod
  }
  if (type) {
    method.type = type
  }
  if (Object.keys(method).length > 0) {
    normalized.method = method
  }

  const headers = parseNameValueText(match?.headersText)
  if (headers.length > 0) {
    normalized.headers = headers
  }

  return normalized
}

const normalizeRules = () => {
  const rules = Array.isArray(formData.value.spec.rules) ? formData.value.spec.rules : []
  const normalized = rules
    .map(rule => {
      const backendRefs = Array.isArray(rule?.backendRefs)
        ? rule.backendRefs.map(normalizeBackendRef).filter(item => item.name)
        : []
      const matches = Array.isArray(rule?.matches)
        ? rule.matches.map(normalizeMatch).filter(item => Object.keys(item).length > 0)
        : []
      const normalizedRule = {}
      if (backendRefs.length > 0) {
        normalizedRule.backendRefs = backendRefs
      }
      if (matches.length > 0) {
        normalizedRule.matches = matches
      }
      return normalizedRule
    })
    .filter(rule => Object.keys(rule).length > 0)

  if (normalized.length > 0) {
    formData.value.spec.rules = normalized
  } else {
    delete formData.value.spec.rules
  }
}

const syncHostnames = () => {
  if (formData.value.spec) {
    formData.value.spec.hostnames = hostnamesInput.value
      .split('\n')
      .map(s => s.trim())
      .filter(s => s)
  }
}

const addParentRef = () => {
  if (!formData.value.spec.parentRefs) formData.value.spec.parentRefs = []
  formData.value.spec.parentRefs.push(createParentRef())
}

const removeParentRef = (idx) => formData.value.spec.parentRefs.splice(idx, 1)

const addBackendRef = (rule) => {
  if (!rule.backendRefs) rule.backendRefs = []
  rule.backendRefs.push(createBackendRef(formData.value?.metadata?.namespace || ''))
}

const addMatch = (rule) => {
  if (!rule.matches) rule.matches = []
  rule.matches.push(createMatch())
}

const addRule = () => {
  if (!formData.value.spec.rules) formData.value.spec.rules = []
  formData.value.spec.rules.push(createRule())
}

const removeRule = (idx) => formData.value.spec.rules.splice(idx, 1)

const sanitizeBackendNumberField = (backendRef, field) => {
  backendRef[field] = String(backendRef[field] ?? '').replace(/[^\d]/g, '')
}

const normalizeBackendNumberField = (backendRef, field) => {
  const config = BACKEND_NUMBER_FIELDS[field]
  if (!config) {
    return
  }
  if (!backendRef[field]) {
    backendRef[field] = config.defaultValue
    return
  }
  const normalized = clampNumber(backendRef[field], config.min, config.max)
  backendRef[field] = normalized === null ? config.defaultValue : String(normalized)
}

const validateBackendRefs = () => {
  const routeRules = Array.isArray(formData.value.spec.rules) ? formData.value.spec.rules : []
  for (let i = 0; i < routeRules.length; i++) {
    const backendRefs = Array.isArray(routeRules[i]?.backendRefs) ? routeRules[i].backendRefs : []
    for (let j = 0; j < backendRefs.length; j++) {
      const backendRef = backendRefs[j]
      if (!backendRef?.name?.trim?.()) {
        return `规则 ${i + 1} 的后端服务 ${j + 1} 缺少服务名称`
      }
      if (!backendRef?.port) {
        return `规则 ${i + 1} 的后端服务 ${j + 1} 缺少端口`
      }
    }
  }
  return ''
}

const FormInitData = () => {
  formData.value = JSON.parse(JSON.stringify(props.form || {}))
  if (!formData.value.apiVersion) formData.value.apiVersion = 'gateway.networking.k8s.io/v1'
  if (!formData.value.kind) formData.value.kind = 'GRPCRoute'
  if (!formData.value.metadata) formData.value.metadata = {}
  if (!formData.value.metadata.labels) formData.value.metadata.labels = {}
  if (!formData.value.metadata.annotations) formData.value.metadata.annotations = {}
  if (!formData.value.spec) formData.value.spec = {}
  if (!Array.isArray(formData.value.spec.parentRefs)) {
    formData.value.spec.parentRefs = []
  } else {
    formData.value.spec.parentRefs = formData.value.spec.parentRefs.map(item => ({
      ...createParentRef(),
      ...item,
      _gatewayKey: `${item?.namespace || formData.value?.metadata?.namespace || ''}/${item?.name || ''}`,
    }))
  }
  if (!formData.value.spec.hostnames) formData.value.spec.hostnames = []
  if (!Array.isArray(formData.value.spec.rules)) {
    formData.value.spec.rules = []
  } else {
    formData.value.spec.rules = formData.value.spec.rules.map(rule => ({
      backendRefs: Array.isArray(rule?.backendRefs) && rule.backendRefs.length > 0
        ? rule.backendRefs.map(item => toEditableBackendRef(item, formData.value?.metadata?.namespace || ''))
        : [createBackendRef(formData.value?.metadata?.namespace || '')],
      matches: Array.isArray(rule?.matches) && rule.matches.length > 0
        ? rule.matches.map(toEditableMatch)
        : [createMatch()],
    }))
  }
  syncBackendRefNamespaces(formData.value?.metadata?.namespace || '')

  if (!props.isEdit) {
    delete formData.value.metadata?.resourceVersion
  }
  delete formData.value.metadata?.uid
  delete formData.value.metadata?.creationTimestamp
  delete formData.value.metadata?.generation
  delete formData.value.status

  hostnamesInput.value = formData.value.spec.hostnames.join('\n')

  readOnly.value = props.readonly
  isEdit.value = props.isEdit
  cluster_id.value = props.cluster
  loadGateways()
}

FormInitData()

watch(
  () => formData.value?.metadata?.namespace,
  async(namespace) => {
    syncBackendRefNamespaces(namespace)
    syncParentRefGatewayKeys(namespace)
    await loadServices(namespace)
    const routeRules = Array.isArray(formData.value?.spec?.rules) ? formData.value.spec.rules : []
    routeRules.forEach(rule => {
      const backendRefs = Array.isArray(rule?.backendRefs) ? rule.backendRefs : []
      backendRefs.forEach(backendRef => handleBackendServiceChange(backendRef))
    })
  },
  { immediate: true }
)

const FormBlock = ref(null)
const closeDialog = () => {
  FormBlock.value?.resetFields()
  emit('close')
}

const enterDialog = async() => {
  syncHostnames()
  const backendRefsError = validateBackendRefs()
  if (backendRefsError) {
    ElMessage({ type: 'error', message: backendRefsError })
    return
  }
  normalizeParentRefs()
  normalizeRules()
  if (formData.value.spec.hostnames && formData.value.spec.hostnames.length === 0) {
    delete formData.value.spec.hostnames
  }
  emit('enter', formData.value)
}
</script>

<style scoped>
.section-card {
  border: 1px solid var(--el-border-color-light);
  border-radius: 6px;
  padding: 16px;
  margin-bottom: 16px;
  background: var(--el-fill-color-light);
}
.section-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 12px;
  font-weight: 600;
  font-size: 14px;
}
.sub-section-title {
  font-size: 13px;
  font-weight: 500;
  color: var(--el-text-color-secondary);
  margin-bottom: 8px;
}
.field-label {
  font-size: 12px;
  color: var(--el-text-color-secondary);
  line-height: 1;
  margin-bottom: 6px;
}
.field-label.required::before {
  content: '*';
  color: var(--el-color-danger);
  margin-right: 4px;
}
</style>
