<template>
  <div style="overflow: hidden;width: 100%;">
    <el-form ref="FormBlock" :model="formData" :rules="rules" label-width="100px">
      <el-form-item label="名称" prop="metadata.name">
        <el-input v-if="OptionType === 'update'" v-model="formData.metadata.name" disabled autocomplete="off" />
        <el-input v-else v-model="formData.metadata.name" autocomplete="off" />
      </el-form-item>
      <el-form-item label="标签" prop="metadata.labels">
        <table style="width: 98%" class="tab-table">
          <tbody>
          <tr>
            <th scope="col" width="48%" align="left">
              <label>键</label>
            </th>
            <th scope="col" width="48%" align="left">
              <label>值</label>
            </th>
            <th align="left" />
          </tr>
          <tr v-for="(row, index) in labels" :key="index">
            <td>
              <el-input v-model="row.key" placeholder="请输入键" />
            </td>
            <td>
              <el-input v-model="row.value" placeholder="请输入值" />
            </td>
            <td>
              <el-button type="danger" link icon="delete" size="small" @click="handleDeleteLabel(index)">删除</el-button>
            </td>
          </tr>
          <tr>
            <td align="left">
              <el-button size="small" type="primary" link icon="plus" @click="handleAddLabel">添加</el-button>
            </td>
          </tr>
          </tbody>
        </table>
      </el-form-item>
      <el-form-item label="注解" prop="metadata.annotations">
        <table style="width: 98%" class="tab-table">
          <tbody>
          <tr>
            <th scope="col" width="48%" align="left">
              <label>键</label>
            </th>
            <th scope="col" width="48%" align="left">
              <label>值</label>
            </th>
            <th align="left" />
          </tr>
          <tr v-for="(row, index) in annotations" :key="index">
            <td>
              <el-input v-model="row.key" placeholder="请输入键" />
            </td>
            <td>
              <el-input v-model="row.value" placeholder="请输入值" />
            </td>
            <td>
              <el-button type="danger" link icon="delete" size="small" @click="handleDeleteAnnotation(index)">删除</el-button>
            </td>
          </tr>
          <tr>
            <td align="left">
              <el-button size="small" type="primary" link icon="plus" @click="handleAddAnnotation">添加</el-button>
            </td>
          </tr>
          </tbody>
        </table>
      </el-form-item>
    </el-form>
    <div class="dialog-footer" align="right">
      <el-button size="small" @click="closeDialog">取 消</el-button>
      <el-button size="small" type="primary" @click="enterDialog">确 定</el-button>
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
import { ref } from 'vue'
const emit = defineEmits(['close', 'enter'])
const props = defineProps({
  form: {
    default: function() {
      return {}
    },
    type: Object
  },
  optype: {
    default: function() {
      return ''
    },
    type: String
  }
})

const rules = ref({
  name: [{ required: true, message: '请输入名称', trigger: 'blur' }],
  kube_config: [{ required: true, message: '请输入集群凭据', trigger: 'blur' }],
  prometheus: [{ required: true, message: '请输入Prometheus地址', trigger: 'blur' }],
})

// 标签格式化
const labels = ref([])
const handleLabelFormatInit = () => {
  if (formData.value.metadata.labels) {
    for (const key in formData.value.metadata.labels) {
      labels.value.push({
        index: Math.random(),
        key: key,
        value: formData.value.metadata.labels[key],
      })
    }
  }
}

// 注解格式化
const annotations = ref([])
const handleAnnotationFormatInit = () => {
  if (formData.value.metadata.annotations) {
    for (const key in formData.value.metadata.annotations) {
      annotations.value.push({
        index: Math.random(),
        key: key,
        value: formData.value.metadata.annotations[key],
      })
    }
  }
}

// 表单数据初始化
const formData = ref({})
const OptionType = ref(false)
const GetFormData = () => {
  formData.value = props.form
  OptionType.value = props.optype
  handleLabelFormatInit()
  handleAnnotationFormatInit()
}

GetFormData()

// 关闭
const closeDialog = () => {
  FormBlock.value.resetFields()
  emit('close')
}

// 标签格式化
const handleLabelFormat = () => {
  if (labels.value.length) {
    const obj = {}
    for (let i = 0; i < labels.value.length; i++) {
      if (labels.value[i].key !== '') {
        obj[labels.value[i].key] = labels.value[i].value
      }
    }
    formData.value.metadata.labels = obj
  }
}

// 标签格式化
const handleAnnotationFormat = () => {
  if (annotations.value.length) {
    const obj = {}
    for (let i = 0; i < annotations.value.length; i++) {
      if (annotations.value[i].key !== '') {
        obj[annotations.value[i].key] = annotations.value[i].value
      }
    }
    formData.value.metadata.annotations = obj
  }
}

// 添加标签
const handleAddAnnotation = () => {
  const item = {
    index: Math.random(),
    key: '',
    value: '',
  }
  annotations.value.push(item)
}

// 删除标签
const handleDeleteAnnotation = (index) => {
  annotations.value.splice(index, 1)
}

// 添加标签
const handleAddLabel = () => {
  const item = {
    index: Math.random(),
    key: '',
    value: '',
  }
  labels.value.push(item)
}

// 删除标签
const handleDeleteLabel = (index) => {
  labels.value.splice(index, 1)
}

// 删除版本信息
const resourceVersionDelete = () => {
  if (formData.value.metadata) {
   delete formData.value.metadata.resourceVersion
  }
}

// 校验并提交
const FormBlock = ref(null)
const enterDialog = async() => {
  FormBlock.value.validate(async valid => {
    if (valid) {
      resourceVersionDelete()
      await handleLabelFormat()
      await handleAnnotationFormat()
      emit('enter', formData.value)
    }
  })
}
</script>
<style scoped>

</style>
