<template>
  <div>
    <el-card>
      <el-form ref="FormBlock" :model="formData" :rules="rules" label-width="120px">
        <el-form-item label="名称" prop="metadata.name">
          <el-input v-model="formData.metadata.name" placeholder="请输入 GatewayClass 名称" :disabled="readOnly || isEdit" />
        </el-form-item>
        <el-form-item label="网关实现" prop="spec.controllerName">
          <el-select
            v-model="formData.spec.controllerName"
            placeholder="请选择或输入 controllerName"
            filterable
            allow-create
            default-first-option
            :disabled="readOnly"
            style="width: 100%"
          >
            <el-option
              v-for="item in controllerOptions"
              :key="item.value"
              :label="`${item.label} (${item.value})`"
              :value="item.value"
            />
          </el-select>
        </el-form-item>
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
  name: 'FormBlock',
}
</script>

<script setup>
import { ref } from 'vue'

const emit = defineEmits(['close', 'enter'])
const props = defineProps({
  form: {
    type: Object,
    default: function() {
      return {}
    }
  },
  readonly: {
    type: Boolean,
    default: function() {
      return false
    }
  },
  isEdit: {
    type: Boolean,
    default: function() {
      return false
    }
  }
})

const controllerOptions = [
  { label: 'Envoy Gateway', value: 'gateway.envoyproxy.io/gatewayclass-controller' },
  { label: 'Istio', value: 'istio.io/gateway-controller' },
  { label: 'Kong', value: 'konghq.com/kic-gateway-controller' },
  { label: 'Cilium', value: 'io.cilium/gateway-controller' },
]

const rules = ref({
  'metadata.name': [{ required: true, message: '请输入名称', trigger: 'blur' }],
  'spec.controllerName': [{ required: true, message: '请选择或输入 controllerName', trigger: 'change' }],
})

const formData = ref({})
const readOnly = ref(false)
const isEdit = ref(false)

const initFormData = () => {
  formData.value = JSON.parse(JSON.stringify(props.form || {}))
  if (!formData.value.apiVersion) formData.value.apiVersion = 'gateway.networking.k8s.io/v1'
  if (!formData.value.kind) formData.value.kind = 'GatewayClass'
  if (!formData.value.metadata) formData.value.metadata = {}
  if (!formData.value.spec) formData.value.spec = {}
  if (!formData.value.spec.controllerName) {
    formData.value.spec.controllerName = 'gateway.envoyproxy.io/gatewayclass-controller'
  }

  if (!props.isEdit) {
    delete formData.value.metadata.resourceVersion
  }
  delete formData.value.metadata.uid
  delete formData.value.metadata.creationTimestamp
  delete formData.value.metadata.generation
  delete formData.value.status

  readOnly.value = props.readonly
  isEdit.value = props.isEdit
}

initFormData()

const FormBlock = ref(null)
const closeDialog = () => {
  FormBlock.value?.resetFields()
  emit('close')
}

const enterDialog = async() => {
  await FormBlock.value.validate((valid) => {
    if (valid) {
      emit('enter', formData.value)
    }
  })
}
</script>

<style scoped>
</style>
