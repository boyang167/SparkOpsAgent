<template>
  <div style="overflow: hidden;width: 100%;">
    <el-form ref="FormBlock" :model="formData" :rules="rules" label-width="100px">
      <el-form-item label="类型" prop="type">
        <el-select v-model="formData.type" filterable placeholder="请选择类型" @change="TypeChange">
          <el-option
            v-for="item in types"
            :key="item"
            :label="item"
            :value="item"
          />
        </el-select>
      </el-form-item>
      <el-form-item label="版本" prop="version">
        <el-select v-model="formData.version" filterable placeholder="请选择类型" style="width: 40%">
          <el-option
            v-for="item in versions"
            :key="item.id"
            :label="item.name"
            :value="item.name"
          />
        </el-select>
      </el-form-item>
      <el-form-item label="名称" prop="name">
        <el-input v-model="formData.name" autocomplete="off" style="width: 70%" />
      </el-form-item>
      <el-form-item label="CPU" prop="cpuCores">
        <el-slider v-model="formData.cpuCores" :max="16" :min="1" show-stops show-tooltip />
        <div class="custom-slider-labels">
          <span v-for="label in sliderLabels" :key="label" class="slider-label">{{ label }}</span>
        </div>
      </el-form-item>
      <el-form-item label="内存" prop="memory">
        <el-slider v-model="formData.memory" :max="16" :min="1" show-stops show-tooltip />
        <div class="custom-slider-labels">
          <span v-for="label in sliderLabels" :key="label" class="slider-label">{{ label }}</span>
        </div>
      </el-form-item>
      <el-form-item label="实例数" prop="replicas">
        <el-input v-model="formData.replicas" :min="1" autocomplete="off" style="width: 20%" type="number" />
        <span class="middlewarespan">( 单位: 个 )</span>
      </el-form-item>
      <el-form-item label="磁盘" prop="disk">
        <el-input v-model="formData.disk" :min="1" autocomplete="off" style="width: 20%" type="number" />
        <span class="middlewarespan">( 单位: Gi )</span>
      </el-form-item>
    </el-form>
    <div align="right" class="dialog-footer">
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
import { ClusterVersionsList } from '@/plugin/kubernetes/api/kubeblocks/clusterversions'
const emit = defineEmits(['close', 'enter'])
const props = defineProps({
  cluster_id: {
    default: function() {
      return 0
    },
    type: Number
  },
  types: {
    default: function() {
      return []
    },
    type: Array
  }
})

const rules = ref({
  name: [{ required: true, message: '请输入名称', trigger: 'blur' }],
  type: [{ required: true, message: '请输选择类型', trigger: 'blur' }],
  version: [{ required: true, message: '请输选择版本', trigger: 'blur' }],
  cpuCores: [{ required: true, message: '请输选择CPU核心数', trigger: 'blur' }],
  memory: [{ required: true, message: '请输选择内存大小', trigger: 'blur' }],
  replicas: [{ required: true, message: '请输输入实例数量', trigger: 'blur' }],
  disk: [{ required: true, message: '请输输入磁盘大小', trigger: 'blur' }],
})

// 标签格式化
const rows = ref([])
// 表单数据初始化
const formData = ref({})
const types = ref([])
const sliderLabels = ref([])
const cluster_id = ref(0)
const versions = ref([])

const GetFormData = () => {
  formData.value.name = ''
  formData.value.version = ''
  formData.value.type = ''
  formData.value.cpuCores = 1
  formData.value.memory = 1
  formData.value.replicas = 1
  formData.value.disk = 20
  types.value = props.types
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

// 选择类型
const TypeChange = async(value) => {
  versions.value = []
  formData.value.version = ''
  const res = await ClusterVersionsList(cluster_id.value)
  if (res.code === 0) {
    if (res.data.items) {
      for (var i = 0; i < res.data.items.length; i++) {
        if (res.data.items[i].metadata.name.toLowerCase().includes(value.toLowerCase())) {
          versions.value.push({ 'id': i, 'name': res.data.items[i].metadata.name })
        }
      }
    }
  }
}
</script>
<style scoped>
.custom-slider {
  display: flex;
  flex-direction: column;
  align-items: center;
  text-align: center;
}

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
