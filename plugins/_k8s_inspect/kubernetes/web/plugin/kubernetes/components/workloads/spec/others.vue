<template>
  <div>
    <el-form ref="form" label-position="top" :model="formData">
      <el-row :gutter="20">
        <el-col :span="12">
          <el-form-item label="重启策略" prop="restartPolicy">
            <form-item-block v-show="isJobs()" v-model="formData.restartPolicy" style="width: 100%" item-type="radio" :radios="restart_policy_list_job" />
            <form-item-block v-show="!isJobs()" v-model="formData.restartPolicy" style="width: 100%" item-type="radio" :radios="restart_policy_list_common" />
          </el-form-item>
        </el-col>
        <el-col :span="12">
          <el-form-item label="终止宽限期(秒)" prop="terminationGracePeriodSeconds">
            <form-item-block v-model.number="formData.terminationGracePeriodSeconds" style="width: 100%" devider-name="秒" item-type="number" />
          </el-form-item>
        </el-col>
      </el-row>
      <el-row :gutter="20">
        <el-col :span="12">
          <el-form-item label="拉取密钥" prop="imagePullSecrets">
            <form-item-block v-model="formData.imagePullSecrets" style="width: 100%" item-type="select2" multiple :selections="secret_list" />
          </el-form-item>
        </el-col>
        <el-col :span="12">
          <el-form-item label="服务账号" prop="serviceAccountName">
            <form-item-block v-model="formData.serviceAccountName" style="width: 100%" item-type="select2" :selections="serviceaccounts_list" />
          </el-form-item>
        </el-col>
      </el-row>
    </el-form>
  </div>
</template>

<script>
export default {
  name: 'OthersBlock',
}
</script>
<script setup>
import FormItemBlock from '@/plugin/kubernetes/components/form-item/index.vue'
import { ref } from 'vue'
const props = defineProps({
  form: {
    default: function() {
      return {}
    },
    type: Object
  },
  serviceaccountsList: {
    default: function() {
      return []
    },
    type: Array
  },
  secretList: {
    default: function() {
      return []
    },
    type: Array
  },
  type: {
    default: function() {
      return ''
    },
    type: String
  },
})

// 判断是否是Job类型
const isJobs = () => {
  if (type.value === 'jobs' || type.value === 'cronjobs') {
    return true
  }
  return false
}

// 数据初始化
const formData = ref([])
const type = ref('')
const restart_policy_list_common = ref([
  { label: 'Always', value: 'Always' },
])
const restart_policy_list_job = ref([
  { label: 'OnFailure', value: 'OnFailure' },
  { label: 'Never', value: 'Never' }]
)
const secret_list = ref([])
const serviceaccounts_list = ref([])
const FormInitData = () => {
  secret_list.value = props.secretList
  serviceaccounts_list.value = props.serviceaccountsList
  if (props.form) {
    formData.value = props.form
    type.value = props.type
    if (!formData.value.restartPolicy) {
      formData.value.restartPolicy = ''
    }
    
    // 处理imagePullSecrets数据格式转换
    if (formData.value.imagePullSecrets && Array.isArray(formData.value.imagePullSecrets)) {
      // 将 [{name: "secret-name"}] 格式转换为 ["secret-name"] 格式
      formData.value.imagePullSecrets = formData.value.imagePullSecrets.map(item => {
        if (typeof item === 'object' && item.name) {
          return item.name
        }
        return item
      })
    }
  }
}

FormInitData()

// 格式化为对象返回父页面
const transformation = (parentFrom) => {
  parentFrom.restartPolicy = formData.value.restartPolicy || undefined
  parentFrom.serviceAccountName = formData.value.serviceAccountName || undefined
  parentFrom.terminationGracePeriodSeconds = formData.value.terminationGracePeriodSeconds || undefined

  const imagePullSecrets = []
  if (formData.value.imagePullSecrets) {
    for (const item of formData.value.imagePullSecrets) {
      // 如果item已经是对象且有name属性，直接使用
      if (typeof item === 'object' && item.name) {
        imagePullSecrets.push({ name: item.name })
      } 
      // 如果item是字符串，包装为{name: item}
      else if (typeof item === 'string') {
        imagePullSecrets.push({ name: item })
      }
      // 如果item是对象但没有name属性，假设整个对象就是name值
      else if (typeof item === 'object') {
        imagePullSecrets.push({ name: item })
      }
    }
  }

  parentFrom.imagePullSecrets = imagePullSecrets.length !== 0 ? imagePullSecrets : undefined
  return
}

// 格式化为对象返回父页面
const transformationformat = async(parentFrom) => {
  transformation(parentFrom)
}

defineExpose({
  transformationformat
})
</script>
<style scoped>

</style>
