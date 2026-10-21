<template>
  <div style="overflow: hidden;width: 100%;">
    <el-form ref="FormBlock" :model="formData" :rules="rules" label-width="100px">
      <el-row>
        <el-col :span="12">
          <el-form-item label="名称" prop="name">
            <el-input v-model="formData.name" autocomplete="off" />
          </el-form-item>
        </el-col>
        <el-col :span="12">
          <el-form-item label="别名" prop="alias">
            <el-input v-model="formData.alias" autocomplete="off" />
          </el-form-item>
        </el-col>
        <el-col :span="24">
          <el-form-item label="描述" prop="description">
            <el-input v-model="formData.description" type="textarea" autocomplete="off" />
          </el-form-item>
        </el-col>
      </el-row>
      <el-form-item label="KubeConfig" prop="kubeConfig">
        <MonacoEditor
            v-model:value="formData.kubeConfig"
            theme="vs"
            :options="optionsData"
            language="yaml"
            width="100%"
            :height="600"
        />
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
  name: 'JoinFormBlock'
}
</script>
<script setup>
import { ref } from 'vue'
import MonacoEditor from 'monaco-editor-vue3'
const emit = defineEmits(['close', 'enter'])
const props = defineProps({
  form: {
    default: function() {
      return {}
    },
    type: Object
  },
})

const formData = ref({})
const rules = ref({
  name: [{ required: true, message: '请输入集群名称', trigger: 'blur' }],
  kubeConfig: [{ required: true, message: '请填入Kubernetes凭据', trigger: 'blur' }],
})

const optionsData = ref(null)
const GetFormData = async() => {
  formData.value = props.form
}

GetFormData()

const Editinit = async() => {
  optionsData.value = {
    value: formData.value.kubeConfig, // 编辑器初始显示文字
    fontSize: 14,
    language: 'yaml', // 此处使用的yaml，其他语言支持自行查阅demo
    theme: 'vs-dark', // 官方自带三种主题vs, hc-black, or vs-dark
    selectOnLineNumbers: true, // 显示行号
    roundedSelection: false,
    readOnly: false, // 只读
    cursorStyle: 'line', // 光标样式
    automaticLayout: true, // 自动布局
    glyphMargin: true, // 字形边缘
    useTabStops: false,
    autoIndent: true, // 自动布局
    quickSuggestionsDelay: 100, // 代码提示延时
  }
}

Editinit()

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

</style>
