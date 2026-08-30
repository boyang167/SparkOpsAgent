<template>
  <div>
    <div class="gva-table-box">
      <div class="gva-btn-list">
        <el-form ref="searchForm" :inline="true" :model="searchInfo">
          <el-form-item label="名称">
            <el-input v-model="searchInfo.keywords" placeholder="名称" />
          </el-form-item>
          <el-form-item>
            <el-button size="small" type="primary" icon="search" @click="onSubmit">查询</el-button>
            <el-button size="small" icon="refresh" @click="onReset">重置</el-button>
            <el-button size="small" type="primary" icon="plus" @click="openDialog">YAML</el-button>
            <el-button size="small" type="primary" icon="plus" @click="openAddDialog">新增</el-button>
          </el-form-item>
        </el-form>
      </div>
      <div v-if="tableData">
        <TableBlock :table-data="tableData" @update="handleUpdate" @yaml_update="handleUpdateYAML" @detail="handleDetail" @delete="handleDelete" />
      </div>
    </div>
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
    <div v-if="dialogYamlVisible">
      <el-dialog v-model="dialogYamlVisible" width="60%" :title="title">
        <template #default>
          <div class="dialog-footer">
            <YamlBlock :data="form" :read-only="readOnly" @close="closeDialog" @enter="enterDialog" />
          </div>
        </template>
      </el-dialog>
    </div>
    <div v-if="dialogAddVisible">
      <el-dialog v-model="dialogAddVisible" width="70%" :title="title">
        <template #default>
          <div class="dialog-footer">
            <FormBlock :cluster="searchInfo.cluster_id" :form="form" :readonly="readOnly" :is-edit="isEdit" @close="closeDialog" @enter="enterDialog" />
          </div>
        </template>
      </el-dialog>
    </div>
    <div v-if="dialogDetailVisible">
      <el-dialog v-model="dialogDetailVisible" width="70%" :title="title">
        <template #default>
          <div class="dialog-footer">
            <DetailBlock :data="form" />
          </div>
        </template>
      </el-dialog>
    </div>
  </div>
</template>

<script>
export default {
  name: 'GRPCRoute',
}
</script>

<script setup>
import {
  GRPCRouteList,
  GRPCRouteGet,
  GRPCRouteCreate,
  GRPCRouteUpdate,
  GRPCRouteDelete,
} from '@/plugin/kubernetes/api/gateway/GRPCRoute'
import { getK8sObject } from '@/plugin/kubernetes/utils/k8s'
import YamlBlock from '@/plugin/kubernetes/components/yaml/index.vue'
import TableBlock from './table.vue'
import DetailBlock from './detail.vue'
import FormBlock from './form.vue'
import { ref } from 'vue'
import { ElMessage } from 'element-plus'

const form = ref({})
const page = ref(1)
const total = ref(0)
const pageSize = ref(10)
const tableData = ref([])
const title = ref('')
const searchInfo = ref({ 'keywords': '', 'namespace': '' })
const readOnly = ref(false)
const isEdit = ref(false)

const onReset = () => {
  searchInfo.value.keywords = ''
}

const onSubmit = () => {
  page.value = 1
  pageSize.value = 10
  getTableData()
}

const handleSizeChange = (val) => {
  pageSize.value = val
  getTableData()
}

const handleCurrentChange = (val) => {
  page.value = val
  getTableData()
}

const getTableData = async() => {
  const table = await GRPCRouteList(searchInfo.value.cluster_id, page.value, pageSize.value, searchInfo.value.namespace, searchInfo.value.keywords)
  if (table.code === 0) {
    tableData.value = table.data.items
    total.value = table.data.total
    page.value = table.data.page
    pageSize.value = table.data.pageSize
  }
}

const dialogYamlVisible = ref(false)
const dialogAddVisible = ref(false)
const dialogDetailVisible = ref(false)
const type = ref('')

const handleUpdate = async(row) => {
  type.value = 'update'
  title.value = '编辑'
  const res = await GRPCRouteGet(searchInfo.value.cluster_id, row.metadata.namespace, row.metadata.name)
  if (res.data.items) {
    if (res.data.items.code) {
      ElMessage({ type: 'error', message: res.data.items.message })
      return
    }
  }
  if (res.code === 0) {
    form.value = res.data.items
    isEdit.value = true
    readOnly.value = false
    dialogAddVisible.value = true
  }
}

const handleUpdateYAML = async(row) => {
  type.value = 'update'
  title.value = '编辑'
  const res = await GRPCRouteGet(searchInfo.value.cluster_id, row.metadata.namespace, row.metadata.name)
  if (res.data.items) {
    if (res.data.items.code) {
      ElMessage({ type: 'error', message: res.data.items.message })
      return
    }
  }
  if (res.code === 0) {
    form.value = res.data.items
    readOnly.value = false
    dialogYamlVisible.value = true
  }
}

const closeDialog = () => {
  dialogYamlVisible.value = false
  dialogAddVisible.value = false
  dialogDetailVisible.value = false
  isEdit.value = false
  form.value = {}
}

const handleDelete = async(row) => {
  row.visible = false
  const res = await GRPCRouteDelete(searchInfo.value.cluster_id, row.metadata.namespace, row.metadata.name)
  if (res.data.items) {
    if (res.data.items.code) {
      ElMessage({ type: 'error', message: res.data.items.message })
      return
    }
  }
  if (res.code === 0) {
    ElMessage({ type: 'success', message: '删除成功' })
    getTableData()
  }
}

const handleOptions = async(res) => {
  if (res.code === 0) {
    ElMessage({ type: 'success', message: res.msg || '操作成功' })
    getTableData()
  }
}

const enterDialog = async(row) => {
  let res
  switch (type.value) {
    case 'create':
      res = await GRPCRouteCreate(searchInfo.value.cluster_id, row.metadata.namespace, row)
      break
    case 'update':
      res = await GRPCRouteUpdate(searchInfo.value.cluster_id, row.metadata.namespace, row.metadata.name, row)
      break
    default:
      res = await GRPCRouteCreate(searchInfo.value.cluster_id, row.metadata.namespace, row)
      break
  }
  if (res.data.items) {
    if (res.data.items.code) {
      ElMessage({ type: 'error', message: res.data.items.message })
      return
    }
  }
  if (res.code === 0) {
    closeDialog()
    handleOptions(res)
  }
}

const openDialog = () => {
  type.value = 'create'
  title.value = '创建'
  form.value = getK8sObject('grpcroutes', '')
  readOnly.value = false
  dialogYamlVisible.value = true
}

const openAddDialog = () => {
  type.value = 'create'
  title.value = '创建'
  form.value = getK8sObject('grpcroutes', '')
  isEdit.value = false
  readOnly.value = false
  dialogAddVisible.value = true
}

const handleDetail = async(row) => {
  const res = await GRPCRouteGet(searchInfo.value.cluster_id, row.metadata.namespace, row.metadata.name)
  if (res.data.items) {
    if (res.data.items.code) {
      ElMessage({ type: 'error', message: res.data.items.message })
      return
    }
  }
  if (res.code === 0) {
    title.value = '详情'
    form.value = res.data.items
    dialogDetailVisible.value = true
  }
}

window.addEventListener('setItemCluster', () => {
  searchInfo.value.cluster_id = parseInt(localStorage.getItem('cluster_id'))
  getTableData()
})
</script>
<style></style>
