<template>
  <div style="overflow: hidden;width: 100%;">
    <el-form ref="FormBlock" :model="formData" :rules="rules" label-width="100px">
      <el-divider> 基本信息 </el-divider>
      <el-form-item label="名称" prop="metadata.name">
        <el-input v-if="option_type === 'create'" v-model="formData.metadata.name" autocomplete="off" style="width: 70%" />
        <el-input v-if="option_type === 'update'" v-model="formData.metadata.name" autocomplete="off" disabled style="width: 70%" />
      </el-form-item>
      <el-form-item label="版本" prop="spec.clusterVersionRef">
        <el-select v-if="option_type === 'create'" v-model="formData.spec.clusterVersionRef" filterable placeholder="请选择版本" style="width: 40%">
          <el-option
            v-for="item in versions"
            :key="item.id"
            :label="item.name"
            :value="item.name"
          />
        </el-select>
        <el-input v-if="option_type === 'update'" v-model="formData.spec.clusterVersionRef" autocomplete="off" disabled style="width: 70%" />
      </el-form-item>
      <el-divider>组件信息</el-divider>
      <div v-for="(componentSpecs, index) in formData.spec.componentSpecs" :key="index">
        <el-form-item label="组件" prop="item.componentDefRef">
          <el-input v-model="componentSpecs.componentDefRef" autocomplete="off" disabled style="width: 70%" />
        </el-form-item>
        <el-form-item label="CPU" prop="resources.limits.cpu">
          <el-slider v-model="componentSpecs.resources.limits.cpu" :max="16" :min="1" show-stops show-tooltip />
          <div class="custom-slider-labels">
            <span v-for="label in sliderLabels" :key="label" class="slider-label">{{ label }}</span>
          </div>
        </el-form-item>
        <el-form-item label="内存" prop="resources.limits.cpu">
          <el-slider v-model="componentSpecs.resources.limits.memory" :max="16" :min="1" show-stops show-tooltip />
          <div class="custom-slider-labels">
            <span v-for="label in sliderLabels" :key="label" class="slider-label">{{ label }}</span>
          </div>
        </el-form-item>
        <el-form-item label="实例数" prop="replicas">
          <el-input
            v-model="componentSpecs.replicas"
            :controls="false"
            autocomplete="off"
            style="width: 20%"
            type="number"
          />
          <span class="middlewarespan">( 单位: 个 )</span>
        </el-form-item>
        <div v-for="(volumeClaimTemplate, vctindex) in componentSpecs.volumeClaimTemplates" :key="vctindex">
          <el-form-item label="磁盘" prop="spec.resources.requests.storage">
            <el-input v-model="volumeClaimTemplate.spec.resources.requests.storage" autocomplete="off" style="width: 20%" />
            <span class="middlewarespan">( 单位: Gi )</span>
          </el-form-item>
        </div>
        <el-divider />
      </div>
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
  cluster_id: {
    default: function() {
      return 0
    },
    type: Number
  },
  form: {
    default: function() {
      return {}
    },
    type: Object
  },
  type: {
    default: function() {
      return ''
    },
    type: String
  },
  versions: {
    default: function() {
      return []
    },
    type: Array
  }
})

const rules = ref({
  'metadata.name': [{ required: true, message: '请输入名称', trigger: 'blur' }],
  'spec.clusterVersionRef': [{ required: true, message: '请选择版本', trigger: 'blur' }],
})

// 内存cpu数值转换
const MemCpuToInt = (str) => {
  if (typeof str === 'string') {
    var stringWithoutLetters = str.replace(/[a-zA-Z]/g, '')
    var result = parseFloat(stringWithoutLetters)
    if (isNaN(result)) {
      return null
    }
  }

  return result
}
// 标签格式化
// 表单数据初始化
const formData = ref({})
const types = ref([])
const sliderLabels = ref([])
const cluster_id = ref(0)
const versions = ref([])
const option_type = ref('')
const GetFormData = () => {
  formData.value = props.form
  if (formData.value.spec.componentSpecs) {
    formData.value.spec.componentSpecs.forEach(item => {
      item.resources.limits.memory = parseInt((MemCpuToInt(item.resources.limits.memory)))
      item.resources.limits.cpu = parseInt((MemCpuToInt(item.resources.limits.cpu)))
    })
  }

  option_type.value = props.type
  versions.value = props.versions
  cluster_id.value = props.cluster_id
  sliderLabels.value = Array.from({ length: 16 }, (_, i) => i + 1)
}

GetFormData()

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

</script>
<style scoped>
.custom-slider-labels {
  display: flex;
  justify-content: space-between;
  width: 100%;
  margin-top: 10px;
}

.slider-label {
  font-size: 12px;
  color: #666;
  flex-grow: 1;
  text-align: center;
}
.middlewarespan {
  margin-left: 5px; /* 可以根据需要调整间距 */
  color: #999; /* 可以根据需要调整颜色 */
}
</style>
