<template>
  <div>
    <el-row>
      <div style="margin-left: auto;">
        <el-button
          size="small"
          icon="refresh"
          @click="onReset"
        >刷新</el-button>
      </div>
    </el-row>
    <el-table :data="tableData">
      <el-table-column
        label="版本"
        prop="version"
      />
      <el-table-column
        label="Deploy User"
        prop="deployUser"
      >
        <template #default="scope">
          {{ scope.row.deployUser.name }}
        </template>
      </el-table-column>
      <el-table-column
        label="状态"
        prop="status"
      >
        <template #default="scope">
          <el-tag
            v-if="scope.row.status ==='running'"
            type="success"
          >Running</el-tag>
          <el-tag
            v-else-if="scope.row.status ==='complete'"
            type="info"
          >Completed</el-tag>
          <el-tag
            v-else
            type="danger"
          >{{ scope.row.status }}</el-tag>
        </template>
      </el-table-column>
      <el-table-column
        label="触发器"
        prop="triggerType"
      />
      <el-table-column
        label="时间"
        prop="createTime"
      >
        <template #default="scope">
          {{ AgeFormat(scope.row.createTime) }}
        </template>
      </el-table-column>
      <el-table-column
        label="环境"
        prop="envName"
      />
      <el-table-column label="操作">
        <template #default="scope">
          <el-button
            size="small"
            type="primary"
            link
            @click="handleDetailRevision(scope.row)"
          >详情</el-button>
          <el-button
            v-if="scope.row.status === 'running'"
            size="small"
            type="primary"
            link
            @click="handleContinue(scope.row)"
          >继续</el-button>
          <el-button
            v-if="scope.row.status === 'running'"
            size="small"
            type="primary"
            link
            @click="handleTerminate(scope.row)"
          >停止</el-button>
          <!--          <el-button-->
          <!--            v-if="scope.row.status !== 'running'"-->
          <!--            size="small"-->
          <!--            type="primary"-->
          <!--            link-->
          <!--            @click="handleRecords(scope.row)"-->
          <!--          >回滚</el-button>-->
        </template>
      </el-table-column>
    </el-table>
    <div class="gva-pagination">
      <el-pagination
        :current-page="page"
        :page-size="pageSize"
        :page-sizes="[10, 30, 50, 100]"
        :total="total"
        layout="total, sizes, prev, pager, next, jumper"
        @current-change="handleCurrentChange"
        @size-change="handleSizeChange"
      />
    </div>

    <div v-if="dialogYAMLVisible">
      <el-dialog
        v-model="dialogYAMLVisible"
        :title="title"
        width="70%"
      >
        <template #default>
          <div class="dialog-footer">
            <MonacoEditor
              v-model:value="RevisionDetail"
              theme="vs"
              :options="optionsData"
              language="yaml"
              width="100%"
              :height="600"
            />
          </div>
        </template>
      </el-dialog>
    </div>
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
  ApplicationRevisionsList,
  ApplicationWorkflowsResume, ApplicationWorkflowsTerminate,
} from '@/plugin/kubernetes/api/kubevela/applications'
import MonacoEditor from 'monaco-editor-vue3'
import { ElMessage } from 'element-plus'

const props = defineProps({
  data: {
    default: function() {
      return {}
    },
    type: Object
  },
})

// 版本列表
const page = ref(1)
const pageSize = ref(10)
const total = ref(0)
const tableData = ref([])
const GetRevisions = async() => {
  const res = await ApplicationRevisionsList(detailData.value.cluster_id, detailData.value.metadata.name, page.value, pageSize.value)
  if (res.code === 0) {
    tableData.value = res.data.revisions
    total.value = res.data.total
  }
}

// 分页
const handleSizeChange = (val) => {
  pageSize.value = val
  GetRevisions()
}

const handleCurrentChange = (val) => {
  page.value = val
  GetRevisions()
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
const dialogYAMLVisible = ref(false)
const title = ref('')
const handleDetailRevision = async(row) => {
  const res = await ApplicationRevisionsGet(detailData.value.cluster_id, detailData.value.metadata.name, row.version)
  if (res.code === 0) {
    RevisionDetail.value = res.data.applyAppConfig
    dialogYAMLVisible.value = true
    title.value = '版本详情:  ' + row.version
    await Editinit()
  }
}

// 回滚版本
const handleRecords = (row) => {
  console.log(row)
}

// 继续
const handleContinue = async(row) => {
  const res = await ApplicationWorkflowsResume(detailData.value.cluster_id, detailData.value.metadata.name, row.workflowName, row.version)
  if (res.code === 0) {
    ElMessage({
      type: 'success',
      message: '操作成功！',
    })
  }
}

// 停止
const handleTerminate = async(row) => {
  const res = await ApplicationWorkflowsTerminate(detailData.value.cluster_id, detailData.value.metadata.name, row.workflowName, row.version)
  if (res.code === 0) {
    ElMessage({
      type: 'success',
      message: '操作成功！',
    })
  }
}

// 刷新列表
const onReset = () => {
  GetRevisions()
}
</script>
<style scoped>

</style>
