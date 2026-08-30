<template>
  <div style="overflow: hidden;width: 100%;">
    <el-form
      ref="FormBlock"
      :model="formData"
      :rules="rules"
      label-width="100px"
    >
      <el-form-item
        label="名称"
        prop="name"
      >
        <el-input
          v-model="formData.name"
          autocomplete="off"
        />
      </el-form-item>
      <el-form-item
        label="凭据类型"
        prop="kube_type"
      >
        <el-radio-group v-model="formData.kube_type">
          <el-radio :value="1">KubeConfig</el-radio>
          <el-radio :value="2">Token</el-radio>
        </el-radio-group>
      </el-form-item>
      <el-form-item
        label="集群凭证"
        prop="kube_config"
      >
        <el-input
          v-model="formData.kube_config"
          autocomplete="off"
          type="textarea"
        />
      </el-form-item>
      <el-form-item
        label="接口地址"
        prop="api_address"
      >
        <el-input
          v-model="formData.api_address"
          autocomplete="off"
          placeholder="请输入Master APi 地址, 例如: https://127.0.0.1:6443"
        />
      </el-form-item>
      <el-divider>
        <span>Ai配置</span>
      </el-divider>
      <el-form-item
          label="接口地址"
          prop="baseURL"
      >
        <el-input
            v-model="formData.baseURL"
            autocomplete="off"
            placeholder="请输入接口地址, 例如: https://api.deepseek.com/v1/chat/completions"
        />
      </el-form-item>
      <el-form-item
          label="ApiKey"
          prop="apiKey"
      >
        <el-input
            v-model="formData.apiKey"
            autocomplete="off"
            type="password"
            placeholder="请输入apiKey"
        />
      </el-form-item>
      <el-form-item
          label="模型名称:"
          prop="modelName"
      >
        <el-input
            v-model="formData.modelName"
            autocomplete="off"
            placeholder="请输入模型名称, 例如: deepseek-chat"
        />
      </el-form-item>
      <el-divider>
        <span>VelaUX多集群管理</span>
      </el-divider>
      <el-form-item
        label="VelaUX地址"
        prop="velaux_url"
      >
        <el-input
          v-model="formData.velaux_url"
          autocomplete="off"
          placeholder="请输入VelaUX接口地址, 例如: http://127.0.0.1:8000"
        />
      </el-form-item>
      <el-form-item
        label="VelaUX用户"
        prop="velaux_user"
      >
        <el-input
          v-model="formData.velaux_user"
          autocomplete="off"
          placeholder="请输入VelaUX认证用户"
        />
      </el-form-item>
      <el-form-item
        label="VelaUX密码"
        prop="velaux_pwd"
      >
        <el-input
          v-model="formData.velaux_pwd"
          :type="velauxlock === 'lock' ? 'password' : 'text'"
          placeholder="请输入VelaUX认证密码"
        >
          <template #suffix>
            <span class="input-icon">
              <el-icon>
                <component
                  :is="velauxlock"
                  @click="velauxchangeLock"
                />
              </el-icon>
            </span>
          </template>
        </el-input>
      </el-form-item>
      <el-divider>
        <span>Prometheus监控</span>
      </el-divider>
      <el-form-item
        label="监控地址"
        prop="prometheus_url"
      >
        <el-input
          v-model="formData.prometheus_url"
          autocomplete="off"
          placeholder="请输入普罗米修斯监控地址, 例如: http://127.0.0.1:9090"
        />
      </el-form-item>
      <el-form-item
        label="认证类型"
        prop="prometheus_auth_type"
      >
        <el-radio-group
          v-model="formData.prometheus_auth_type"
          @change="TypeChange"
        >
          <el-radio :value="1">无</el-radio>
          <el-radio :value="2">BaseAuth</el-radio>
        </el-radio-group>
      </el-form-item>
      <div v-if="prometheusTypeStatus">
        <el-form-item
          v-if="formData.prometheus_auth_type === 2"
          label="用户名"
          prop="prometheus_user"
        >
          <el-input
            v-model="formData.prometheus_user"
            autocomplete="off"
            placeholder="请输入普罗米修斯认证用户"
          />
        </el-form-item>
        <el-form-item
          v-if="formData.prometheus_auth_type === 2"
          label="密码"
          prop="prometheus_pwd"
        >
          <el-input
            v-model="formData.prometheus_pwd"
            :type="prometheuslock === 'lock' ? 'password' : 'text'"
            placeholder="请输入普罗米修斯认证密码"
          >
            <template #suffix>
              <span class="input-icon">
                <el-icon>
                  <component
                    :is="prometheuslock"
                    @click="prometheuschangeLock"
                  />
                </el-icon>
              </span>
            </template>
          </el-input>
        </el-form-item>
      </div>
    </el-form>
    <div
      class="dialog-footer"
      align="right"
    >
      <el-button
        size="small"
        @click="closeDialog"
      >取 消</el-button>
      <el-button
        size="small"
        type="primary"
        @click="enterDialog"
      >确 定</el-button>
    </div>
  </div>
</template>

<script>
export default {
  name: 'FormBlock',
  emits: {
    close: () => {
      return true
    },
    enter: (value) => {
      if (value.length === 1) {
        return true
      } else {
        return false
      }
    },
  }
}
</script>
<script setup>
import { ref, watch } from 'vue'
import yaml from 'js-yaml'
const emit = defineEmits(['close', 'enter'])
const props = defineProps({
  form: {
    default: function() {
      return {}
    },
    type: Object
  },
})

const rules = ref({
  name: [{ required: true, message: '请输入名称', trigger: 'blur' }],
  kube_type: [{ required: true, message: '请输选择凭据类型', trigger: 'blur' }],
  kube_config: [{ required: true, message: '请输入集群凭据', trigger: 'blur' }],
  api_address: [{ required: true, message: '请输入Master接口地址', trigger: 'blur' }],
  prometheus_auth_type: [{ required: true, message: '请选择认证类型', trigger: 'blur' }],
  prometheus_user: [{ required: true, message: '请输入用户名', trigger: 'blur' }],
  prometheus_pwd: [{ required: true, message: '请输入密码', trigger: 'blur' }],
})

// 表单数据初始化
const formData = ref({})
const prometheusTypeStatus = ref(false)
const GetFormData = () => {
  formData.value = {
    ...(props.form || {})
  }
  if (formData.value.prometheus_auth_type === 2) {
    prometheusTypeStatus.value = true
  } else {
    prometheusTypeStatus.value = false
  }

  if (!formData.value.prometheus_auth_type) {
    formData.value.prometheus_auth_type = 1
  }

  if (!formData.value.kube_type) {
    formData.value.kube_type = 1
  }
}

GetFormData()

const normalizeKubeConfigText = (text) => {
  if (!text) return ''
  return text
    .replace(/```(?:yaml)?/gi, '')
    .replace(/:\s*`([^`]+)`/g, ': $1')
    .replace(/\r\n/g, '\n')
    .trim()
}

const stripWrappedValue = (value) => {
  if (!value) return ''
  return value.trim().replace(/^['"`](.*)['"`]$/, '$1')
}

const extractClusterInfoFromText = (kubeConfigText) => {
  const normalizedText = normalizeKubeConfigText(kubeConfigText)
  const lines = normalizedText.split('\n')
  const currentContextMatch = normalizedText.match(/^\s*current-context:\s*(.+)\s*$/m)
  const currentContextName = stripWrappedValue(currentContextMatch?.[1] || '')

  let clusterName = ''
  let apiAddress = ''
  let activeContextName = ''
  let currentContextClusterName = ''
  let activeClusterName = ''

  for (const rawLine of lines) {
    const line = rawLine.trim()
    const contextNameMatch = line.match(/^-?\s*name:\s*(.+)$/)
    if (contextNameMatch && !activeClusterName) {
      activeContextName = stripWrappedValue(contextNameMatch[1])
      continue
    }

    const contextClusterMatch = line.match(/^cluster:\s*(.+)$/)
    if (activeContextName && contextClusterMatch) {
      const contextClusterName = stripWrappedValue(contextClusterMatch[1])
      if (activeContextName === currentContextName) {
        currentContextClusterName = contextClusterName
        break
      }
    }
  }

  activeClusterName = ''
  for (const rawLine of lines) {
    const line = rawLine.trim()
    const clusterNameMatch = line.match(/^-?\s*name:\s*(.+)$/)
    if (clusterNameMatch) {
      activeClusterName = stripWrappedValue(clusterNameMatch[1])
      if (!clusterName) {
        clusterName = activeClusterName
      }
      continue
    }

    const serverMatch = line.match(/^server:\s*(.+)$/)
    if (serverMatch && activeClusterName) {
      const server = stripWrappedValue(serverMatch[1])
      if (!apiAddress) {
        apiAddress = server
      }
      if (currentContextClusterName && activeClusterName === currentContextClusterName) {
        clusterName = currentContextClusterName
        apiAddress = server
        break
      }
    }
  }

  if (currentContextClusterName) {
    clusterName = currentContextClusterName
  }

  return { clusterName, apiAddress }
}

const extractClusterInfoFromKubeConfig = (kubeConfigText) => {
  if (!kubeConfigText) {
    return { clusterName: '', apiAddress: '' }
  }

  const normalizedText = normalizeKubeConfigText(kubeConfigText)

  try {
    const config = yaml.load(normalizedText)
    const contexts = Array.isArray(config?.contexts) ? config.contexts : []
    const clusters = Array.isArray(config?.clusters) ? config.clusters : []

    let clusterName = ''
    let apiAddress = ''

    if (config?.['current-context']) {
      const currentContext = contexts.find(item => item.name === config['current-context'])
      clusterName = currentContext?.context?.cluster || ''
    }

    if (!clusterName && clusters.length > 0) {
      clusterName = clusters[0]?.name || ''
    }

    if (clusterName) {
      const cluster = clusters.find(item => item.name === clusterName)
      apiAddress = stripWrappedValue(cluster?.cluster?.server || '')
    } else if (clusters.length > 0) {
      apiAddress = stripWrappedValue(clusters[0]?.cluster?.server || '')
    }

    return { clusterName, apiAddress }
  } catch (e) {
    return extractClusterInfoFromText(normalizedText)
  }
}

const syncKubeConfigFields = (kubeConfigText) => {
  if (!kubeConfigText || formData.value.kube_type !== 1) return

  try {
    const { clusterName, apiAddress } = extractClusterInfoFromKubeConfig(kubeConfigText)
    if (clusterName) {
      formData.value.name = clusterName
    }
    if (apiAddress) {
      formData.value.api_address = apiAddress
    }
  } catch (e) {
    console.error('Failed to parse kubeconfig:', e)
  }
}

watch(() => props.form, () => {
  GetFormData()
  syncKubeConfigFields(formData.value.kube_config)
}, { deep: true, immediate: true })

watch(() => formData.value.kube_config, (val) => {
  syncKubeConfigFields(val)
}, { immediate: true })

// 关闭
const closeDialog = () => {
  FormBlock.value.resetFields()
  emit('close')
}

// 校验并提交
const FormBlock = ref(null)
const enterDialog = async() => {
  FormBlock.value.validate(async valid => {
    if (valid) {
      emit('enter', formData.value)
    }
  })
}

// 普罗米修斯认证类型改变
const TypeChange = (type) => {
  if (type === 2) {
    prometheusTypeStatus.value = true
  }
}

// 查看密码
const prometheuslock = ref('lock')
const prometheuschangeLock = () => {
  prometheuslock.value = prometheuslock.value === 'lock' ? 'unlock' : 'lock'
}

// 查看密码
const velauxlock = ref('lock')
const velauxchangeLock = () => {
  velauxlock.value = velauxlock.value === 'lock' ? 'unlock' : 'lock'
}
</script>
<style scoped>

</style>
