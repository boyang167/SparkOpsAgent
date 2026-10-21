<template>
  <div>
    <el-card v-show="tableData">
      <el-table :data="tableData">
        <el-table-column
          align="left"
          label="名称"
          prop="name"
        >
          <template #default="scope">
            <span
              class="operate-span-primary"
              @click="handleDetail(scope.row)"
            >{{ scope.row.name }}</span>
          </template>
        </el-table-column>
        <el-table-column
          label="环境"
          prop="envName"
        />
        <el-table-column
          label="执行模式"
          prop="mode"
        />

        <el-table-column
          label="步骤数量"
          prop="steps"
        >
          <template #default="scope">
            {{ StepsNumber(scope.row.steps) }}
          </template>
        </el-table-column>
        <el-table-column
          label="创建时间"
          prop="createTime"
        >
          <template #default="scope">
            {{ AgeFormat(scope.row.createTime) }}
          </template>
        </el-table-column>
        <el-table-column
          label="更新时间"
          prop="createTime"
        >
          <template #default="scope">
            {{ AgeFormat(scope.row.updateTime) }}
          </template>

        </el-table-column>
      </el-table>
    </el-card>
  </div>
</template>

<script>
export default {
  name: 'DetailRevisions',
}
</script>
<script setup>
import { ref } from 'vue'
import { AgeFormat } from '@/plugin/kubernetes/utils/age'
import {
  ApplicationRevisionsGet,
  ApplicationWorkflowsList,
} from '@/plugin/kubernetes/api/kubevela/applications'

const props = defineProps({
  data: {
    default: function() {
      return {}
    },
    type: Object
  },
})

// 版本列表
const tableData = ref([])
const GetRevisions = async() => {
  const res = await ApplicationWorkflowsList(detailData.value.cluster_id, detailData.value.metadata.name)
  if (res.code === 0) {
    tableData.value = res.data.workflows
  }
}

// 数据初始化
const detailData = ref({})
const detailInitData = async() => {
  detailData.value = props.data
  await GetRevisions()
}

detailInitData()

const optionsData = ref(null)
const Editinit = async() => {
  optionsData.value = {
    value: RevisionDetail.value, // 编辑器初始显示文字
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

// 版本详情
const RevisionDetail = ref('')
const title = ref('')
const handleDetail = async(row) => {
  title.value = '工作流详情:  ' + row.name
  console.log(row)
}

const StepsNumber = (steps) => {
  return steps.length
}
</script>
<style scoped>
.operate-span-primary {
  color: var(--el-color-primary);
  cursor: pointer;
  font-weight:bold;
}
</style>
