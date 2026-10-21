<template>
  <div>
    <el-card class="el-card">
      <el-form ref="FormBlock" :model="formData" :rules="rules" label-width="100px">
        <el-row :gutter="24">
          <el-col :span="8">
            <el-form-item label="名称" prop="metadata.name">
              <form-item-block v-if="readOnly || isEdit" v-model="formData.metadata.name" style="width: 100%" disabled item-type="input" />
              <form-item-block v-else v-model="formData.metadata.name" placeholder="请输入名称" item-type="input" style="width: 100%" />
            </el-form-item>
          </el-col>
          <el-col :span="8">
            <el-form-item label="命名空间" required prop="metadata.namespace">
              <form-item-block v-if="readOnly || isEdit" v-model="formData.metadata.namespace" style="width: 100%" disabled item-type="select2" />
              <NsSelect v-else :cluster="cluster_id" :namespace.sync="formData.metadata.namespace" @enter="enterNamespace" />
            </el-form-item>
          </el-col>
          <el-col :span="8">
            <el-form-item label="GatewayClass" prop="spec.gatewayClassName">
              <el-input v-model="formData.spec.gatewayClassName" :disabled="readOnly" placeholder="请输入 GatewayClass 名称，例如 eg" />
            </el-form-item>
          </el-col>
        </el-row>
        <el-col :span="24">
          <el-tabs
            ref="tabs"
            v-model="activeName"
            tab-position="top"
            type="border-card"
            @tab-click="handleClick"
          >
            <el-tab-pane label="监听器" name="listeners">
              <div v-for="(listener, index) in formData.spec.listeners" :key="index" class="listener-card">
                <div class="listener-header">
                  <span>监听器 {{ index + 1 }}</span>
                  <el-button v-if="!readOnly" size="small" type="danger" link @click="removeListener(index)">删除</el-button>
                </div>
                <el-row :gutter="20">
                  <el-col :span="6">
                    <el-form-item :label="index === 0 ? '名称' : ''" :prop="`spec.listeners.${index}.name`" :rules="listenerRules.name">
                      <el-input v-model="listener.name" :disabled="readOnly" placeholder="例如 http / foo / bar" />
                    </el-form-item>
                  </el-col>
                  <el-col :span="6">
                    <el-form-item :label="index === 0 ? '协议' : ''" :prop="`spec.listeners.${index}.protocol`" :rules="listenerRules.protocol">
                      <el-select v-model="listener.protocol" :disabled="readOnly" style="width: 100%" placeholder="请选择协议" @change="updateAllowedRouteKinds(listener)">
                        <el-option v-for="item in protocolOptions" :key="item" :label="item" :value="item" />
                      </el-select>
                    </el-form-item>
                  </el-col>
                  <el-col :span="6">
                    <el-form-item :label="index === 0 ? '端口' : ''" :prop="`spec.listeners.${index}.port`" :rules="listenerRules.port">
                      <el-input-number v-model="listener.port" :disabled="readOnly" :min="1" :max="65535" style="width: 100%" />
                    </el-form-item>
                  </el-col>
                  <el-col :span="6">
                    <el-form-item :label="index === 0 ? '域名' : ''">
                      <el-input v-model="listener.hostname" :disabled="readOnly" placeholder="可选，例如 example.com" />
                    </el-form-item>
                  </el-col>
                </el-row>
                <el-row :gutter="20">
                  <el-col :span="12">
                    <el-form-item label="允许路由类型">
                      <el-select
                        v-model="listener.allowedRouteKinds"
                        multiple
                        collapse-tags
                        :disabled="readOnly"
                        style="width: 100%"
                        placeholder="请选择允许的 Route 类型"
                        @change="onRouteKindsChange(listener)"
                      >
                        <el-option
                          v-for="kind in (protocolRouteKinds[listener.protocol] || [])"
                          :key="kind"
                          :label="kind"
                          :value="kind"
                        />
                      </el-select>
                    </el-form-item>
                  </el-col>
                  <el-col :span="12">
                    <el-form-item label="允许命名空间">
                      <el-select
                        v-model="listener.allowedNamespaceFrom"
                        :disabled="readOnly"
                        allow-create
                        filterable
                        default-first-option
                        style="width: 100%"
                        placeholder="输入或选择"
                      >
                        <el-option label="Same" value="Same" />
                        <el-option label="All" value="All" />
                        <el-option label="Selector" value="Selector" />
                      </el-select>
                    </el-form-item>
                  </el-col>
                </el-row>
                <el-row v-if="listener.allowedNamespaceFrom === 'Selector'">
                   <el-col :span="24">
                     <el-form-item label="标签选择器">
                       <div v-for="key in Object.keys(listener.namespaceSelector?.matchLabels || {})" :key="key" style="margin-bottom: 8px; display: flex; gap: 8px; align-items: center;">
                         <span style="width: 60px; color: var(--el-text-color-secondary); font-size: 12px;">{{ key }}</span>
                         <el-input v-model="listener.namespaceSelector.matchLabels[key]" :disabled="readOnly" placeholder="value" style="flex: 1;" />
                         <el-button :disabled="readOnly" icon="delete" @click="delete listener.namespaceSelector.matchLabels[key]" />
                       </div>
                       <div v-if="!readOnly" style="margin-bottom: 8px; display: flex; gap: 8px; align-items: center;">
                         <el-input v-model="listener._newLabelKey" placeholder="key" style="width: 160px;" @keydown.enter.prevent="addMatchLabel(listener)" />
                         <el-input v-model="listener._newLabelValue" placeholder="value" style="flex: 1;" @keydown.enter.prevent="addMatchLabel(listener)" />
                         <el-button :disabled="readOnly" @click="addMatchLabel(listener)">添加</el-button>
                       </div>
                       <el-divider v-if="!readOnly" style="margin: 12px 0;" />
                       <div v-if="!readOnly">
                         <div v-for="(expr, idx) in (listener.namespaceSelector?.matchExpressions || [])" :key="idx" style="margin-bottom: 8px; display: flex; gap: 8px; align-items: center;">
                           <el-input v-model="expr.key" :disabled="readOnly" placeholder="key" style="width: 140px;" />
                           <el-select v-model="expr.operator" :disabled="readOnly" style="width: 130px;">
                             <el-option label="In" value="In" />
                             <el-option label="NotIn" value="NotIn" />
                             <el-option label="Exists" value="Exists" />
                             <el-option label="DoesNotExist" value="DoesNotExist" />
                           </el-select>
                           <el-input v-if="expr.operator === 'In' || expr.operator === 'NotIn'" v-model="expr.values" placeholder="逗号分隔多个值" style="width: 220px;" />
                           <el-button :disabled="readOnly" icon="delete" @click="listener.namespaceSelector.matchExpressions.splice(idx, 1)" />
                         </div>
                         <el-button :disabled="readOnly" size="small" @click="addMatchExpression(listener)">添加表达式</el-button>
                       </div>
                     </el-form-item>
                   </el-col>
                 </el-row>
                <el-row v-if="listener.protocol === 'HTTPS' || listener.protocol === 'TLS'" :gutter="20">
                  <el-col :span="12">
                    <el-form-item label="TLS 模式">
                      <el-select v-model="listener.tlsMode" :disabled="readOnly" style="width: 100%" placeholder="请选择 TLS 模式">
                        <el-option label="Terminate" value="Terminate" />
                        <el-option label="Passthrough" value="Passthrough" />
                      </el-select>
                    </el-form-item>
                  </el-col>
                  <el-col :span="12">
                    <el-form-item label="证书 Secret">
                      <el-input v-model="listener.tlsSecretName" :disabled="readOnly" placeholder="Secret 名称，例如 api-example-tls" />
                    </el-form-item>
                  </el-col>
                </el-row>
              </div>
              <el-button v-if="!readOnly" size="small" type="primary" plain @click="addListener">新增监听器</el-button>
            </el-tab-pane>
            <el-tab-pane label="高级配置" name="advanced">
              <el-form-item label="地址类型">
                <el-select v-model="addressType" :disabled="readOnly" style="width: 240px" placeholder="可选">
                  <el-option label="不设置" value="" />
                  <el-option label="Hostname" value="Hostname" />
                  <el-option label="IPAddress" value="IPAddress" />
                  <el-option label="NamedAddress" value="NamedAddress" />
                </el-select>
              </el-form-item>
              <el-form-item label="地址值" v-if="addressType">
                <el-input v-model="addressValue" :disabled="readOnly" placeholder="例如 192.168.1.10 或 gateway.example.com" />
              </el-form-item>
              <div style="font-size: 12px; color: #909399; line-height: 1.6;">
                适用于类似 `gatewayClassName + listeners + allowedRoutes` 的 Gateway 场景。
              </div>
            </el-tab-pane>
            <el-tab-pane name="Labels" label="标签/注解">
              <KVTableBlock ref="labels" :title="'标签'" :form.sync="formData.metadata.labels" />
              <KVTableBlock ref="annotations" :title="'注解'" :form.sync="formData.metadata.annotations" />
            </el-tab-pane>
          </el-tabs>
        </el-col>

      </el-form>
      <div class="grid-content bg-purple-light">
        <div style="float: right;margin-top: 10px">
          <el-button size="small" @click="closeDialog">取 消</el-button>
          <el-button size="small" type="primary" @click="enterDialog">确 定</el-button>
        </div>
      </div>
    </el-card>
  </div>
</template>

<script>
export default {
  name: 'FormBlock',
}
</script>
<script setup>
import { ref } from 'vue'
import FormItemBlock from '@/plugin/kubernetes/components/form-item/index.vue'
import NsSelect from '@/plugin/kubernetes/components/namespace/select-ns.vue'
import KVTableBlock from '@/plugin/kubernetes/components/configuration/key-value.vue'

const emit = defineEmits(['close', 'enter'])
const props = defineProps({
  form: {
    default: function() {
      return {}
    },
    type: Object
  },
  cluster: {
    default: function() {
      return 0
    },
    type: Number
  },
  readonly: {
    default: function() {
      return false
    },
    type: Boolean
  },
  isEdit: {
    default: function() {
      return false
    },
    type: Boolean
  }
})

// 数据初始化
const formData = ref({})
const cluster_id = ref({})
const activeName = ref('listeners')
const readOnly = ref(false)
const addressType = ref('')
const addressValue = ref('')
const protocolOptions = ['HTTP', 'HTTPS', 'TLS', 'TCP', 'UDP']
const rules = ref({
  'metadata.name': [{ required: true, message: '请输入名称', trigger: 'blur' }],
  'metadata.namespace': [{ required: true, message: '请选择命名空间', trigger: 'blur' }],
  'spec.gatewayClassName': [{ required: true, message: '请输入 GatewayClass 名称', trigger: 'blur' }],
})
const listenerRules = {
  name: [{ required: true, message: '请输入监听器名称', trigger: 'blur' }],
  protocol: [{ required: true, message: '请选择协议', trigger: 'change' }],
  port: [{ required: true, message: '请输入端口', trigger: 'change' }],
}

const createListener = () => ({
  name: '',
  protocol: '',
  port: undefined,
  hostname: '',
  allowedRouteKinds: [],
  allowedNamespaceFrom: '',
  namespaceSelector: { matchLabels: {}, matchExpressions: [] },
  tlsMode: '',
  tlsSecretName: '',
})

const protocolRouteKinds = {
  HTTP: ['HTTPRoute', 'GRPCRoute'],
  HTTPS: ['HTTPRoute', 'GRPCRoute'],
  TLS: ['TLSRoute'],
  TCP: ['TCPRoute'],
  UDP: ['UDPRoute'],
}

const updateAllowedRouteKinds = (listener) => {
  if (listener.protocol && protocolRouteKinds[listener.protocol]) {
    const validKinds = protocolRouteKinds[listener.protocol]
    listener.allowedRouteKinds = listener.allowedRouteKinds.filter(kind => validKinds.includes(kind))
    if (listener.allowedRouteKinds.length === 0) {
      listener.allowedRouteKinds = [...validKinds]
    }
  } else {
    listener.allowedRouteKinds = []
  }
}

const onRouteKindsChange = (listener) => {
  const validKinds = protocolRouteKinds[listener.protocol] || []
  listener.allowedRouteKinds = listener.allowedRouteKinds.filter(kind => validKinds.includes(kind))
}

const addMatchLabel = (listener) => {
  const key = listener._newLabelKey
  const value = listener._newLabelValue
  if (key && key.trim()) {
    if (!listener.namespaceSelector) {
      listener.namespaceSelector = { matchLabels: {}, matchExpressions: [] }
    }
    if (!listener.namespaceSelector.matchLabels) {
      listener.namespaceSelector.matchLabels = {}
    }
    listener.namespaceSelector.matchLabels[key.trim()] = value || ''
    listener._newLabelKey = ''
    listener._newLabelValue = ''
  }
}

const addMatchExpression = (listener) => {
  if (!listener.namespaceSelector) {
    listener.namespaceSelector = { matchLabels: {}, matchExpressions: [] }
  }
  if (!listener.namespaceSelector.matchExpressions) {
    listener.namespaceSelector.matchExpressions = []
  }
  listener.namespaceSelector.matchExpressions.push({ key: '', operator: 'In', values: '' })
}

const addListener = () => {
  formData.value.spec.listeners.push(createListener())
}

const FormInitData = () => {
  formData.value = JSON.parse(JSON.stringify(props.form || {}))
  if (!formData.value.metadata) formData.value.metadata = {}
  if (!formData.value.metadata.labels) formData.value.metadata.labels = {}
  if (!formData.value.metadata.annotations) formData.value.metadata.annotations = {}
  if (!formData.value.spec) formData.value.spec = {}
  if (!formData.value.spec.gatewayClassName) formData.value.spec.gatewayClassName = ''
  if (!Array.isArray(formData.value.spec.listeners) || formData.value.spec.listeners.length === 0) {
    formData.value.spec.listeners = [createListener()]
  } else {
    formData.value.spec.listeners = formData.value.spec.listeners.map(listener => {
      const extracted = {
        ...createListener(),
        ...listener,
        allowedNamespaceFrom: typeof listener?.allowedRoutes?.namespaces?.from === 'string'
          ? listener.allowedRoutes.namespaces.from
          : '',
        namespaceSelector: {
          matchLabels: JSON.parse(JSON.stringify(listener?.allowedRoutes?.namespaces?.selector?.matchLabels || {})),
          matchExpressions: JSON.parse(JSON.stringify(listener?.allowedRoutes?.namespaces?.selector?.matchExpressions || [])),
        },
        tlsMode: listener?.tls?.mode || '',
        tlsSecretName: listener?.tls?.certificateRefs?.[0]?.name || '',
      }
      updateAllowedRouteKinds(extracted)
      return extracted
    })
  }
  if (Array.isArray(formData.value.spec.addresses) && formData.value.spec.addresses.length > 0) {
    addressType.value = formData.value.spec.addresses[0].type || ''
    addressValue.value = formData.value.spec.addresses[0].value || ''
  }
  cluster_id.value = props.cluster
  readOnly.value = props.readonly
}

FormInitData()

// 命名空间选择
const enterNamespace = async(value) => {
  if (formData.value.metadata) {
    formData.value.metadata.namespace = value
  }
}

// 关闭
const FormBlock = ref(null)
const closeDialog = () => {
  FormBlock.value.resetFields()
  emit('close')
}

const removeListener = (index) => {
  formData.value.spec.listeners.splice(index, 1)
}

const normalizeListeners = async() => {
  for (let i = 0; i < formData.value.spec.listeners.length; i++) {
    const listener = formData.value.spec.listeners[i]
    const allowedRoutes = {}
    if (listener.allowedRouteKinds && listener.allowedRouteKinds.length > 0) {
      allowedRoutes.kinds = listener.allowedRouteKinds.map(kind => ({ kind }))
    }
    if (listener.allowedNamespaceFrom) {
      allowedRoutes.namespaces = { from: listener.allowedNamespaceFrom }
      if (listener.allowedNamespaceFrom === 'Selector' && listener.namespaceSelector) {
        const selector = {}
        const labels = listener.namespaceSelector.matchLabels || {}
        const labelKeys = Object.keys(labels)
        if (labelKeys.length > 0) {
          selector.matchLabels = {}
          labelKeys.forEach(key => {
            if (labels[key] !== undefined && labels[key] !== '') {
              selector.matchLabels[key] = labels[key]
            }
          })
        }
        const expressions = listener.namespaceSelector.matchExpressions || []
        const validExprs = expressions.filter(e => e.key && e.operator)
          .map(e => {
            const expr = { key: e.key, operator: e.operator }
            if ((e.operator === 'In' || e.operator === 'NotIn') && e.values) {
              expr.values = (typeof e.values === 'string' ? e.values : '').split(',').map(v => v.trim()).filter(v => v)
            }
            return expr
          })
        if (validExprs.length > 0) {
          selector.matchExpressions = validExprs
        }
        if (Object.keys(selector).length > 0) {
          allowedRoutes.namespaces.selector = selector
        }
      }
    }

    if (listener.tlsMode || listener.tlsSecretName) {
      const tls = {}
      if (listener.tlsMode) tls.mode = listener.tlsMode
      if (listener.tlsSecretName) {
        tls.certificateRefs = [{ kind: 'Secret', name: listener.tlsSecretName }]
      }
      listener.tls = tls
    }

    if (Object.keys(allowedRoutes).length > 0) {
      listener.allowedRoutes = allowedRoutes
    }
    delete listener.allowedRouteKinds
    delete listener.allowedNamespaceFrom
    delete listener.namespaceSelector
    delete listener._newLabelKey
    delete listener._newLabelValue
    delete listener.tlsMode
    delete listener.tlsSecretName
    if (!listener.hostname) {
      delete listener.hostname
    }
  }
}

// 提交
const labels = ref(null)
const annotations = ref(null)
const enterDialog = async() => {
  if (!formData.value.spec.listeners || formData.value.spec.listeners.length === 0) {
    activeName.value = 'listeners'
    return
  }

  await labels.value.transformationformat(formData.value, 'labels')
  await annotations.value.transformationformat(formData.value, 'annotations')
  await normalizeListeners()

  if (addressType.value && addressValue.value) {
    formData.value.spec.addresses = [{
      type: addressType.value,
      value: addressValue.value
    }]
  } else {
    delete formData.value.spec.addresses
  }

  emit('enter', formData.value)
}

// 切换标签
const handleClick = (tab) => {
  activeName.value = tab.name
}
</script>
<style scoped>
.listener-card {
  padding: 16px;
  margin-bottom: 16px;
  border: 1px solid var(--el-border-color-light);
  border-radius: 6px;
}

.listener-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 12px;
  font-weight: 600;
}


</style>
