<template>
  <div>
    <warning-bar title="注：从平台内删除集群同时会删除通过平台创建的授权，请慎重操作 。" />
    <div class="gva-search-box">
      <el-form
        ref="searchForm"
        :inline="true"
        :model="searchInfo"
      >
        <el-form-item label="名称">
          <el-input
            v-model="searchInfo.name"
            placeholder="名称"
          />
        </el-form-item>
        <el-form-item>
          <el-button
            size="small"
            type="primary"
            icon="search"
            @click="onSubmit"
          >查询</el-button>
          <el-button
            size="small"
            icon="refresh"
            @click="onReset"
          >重置</el-button>
        </el-form-item>
      </el-form>
    </div>
    <div class="gva-table-box">
      <div class="gva-btn-list">
        <el-button
          size="small"
          type="primary"
          icon="plus"
          @click="openDialog('addApi')"
        >新增</el-button>
        <el-button
          size="small"
          icon="delete"
          @click="onDelete"
        >删除</el-button>
      </div>
      <div v-if="tableData">
        <TableBlock
          :table-data="tableData"
          @update="handleUpdate"
          @search="handlesortChange"
          @select="handleSelectionChange"
          @delete="handleDelete"
          @rbac="handleRbacManage"
          @associat="handleAssociat"
        />
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
    <div v-if="dialogFormVisible">
      <el-dialog
        v-model="dialogFormVisible"
        width="40%"
        :title="title"
        :show-close="false"
        :close-on-press-escape="false"
        :close-on-click-modal="false"
      >
        <FormBlock
          :form="form"
          @close="closeDialog"
          @enter="enterDialog"
        />
      </el-dialog>
    </div>

    <div v-if="dialogRbacFormVisible">
      <el-dialog
        v-model="dialogRbacFormVisible"
        width="70%"
        :title="title"
        :close-on-press-escape="false"
        :close-on-click-modal="false"
      >
        <RbacFormBlock
          :form="form"
          @close="closeDialog"
          @enter="enterDialog"
        />
      </el-dialog>
    </div>

    <div v-if="dialogManagementsFormVisible">
      <el-dialog
        v-model="dialogManagementsFormVisible"
        width="70%"
        :title="title"
        :close-on-press-escape="false"
        :close-on-click-modal="false"
      >
        <ManagementBlock
          :form="form"
          @close="closeDialog"
          @enter="enterDialog"
        />
      </el-dialog>
    </div>
  </div>
</template>

<script>

export default {
  name: 'Index',
}
</script>

<script setup>
import {
  getClustersList,
  CreateCluster,
  UpdateCluster,
  DeleteCluster,
  DeleteClusterByIds,
  getClustersById,
} from '@/plugin/kubernetes/api/clusters'
import FormBlock from './form.vue'
import RbacFormBlock from '@/plugin/kubernetes/components/clusters/rbac.vue'
import ManagementBlock from '@/plugin/kubernetes/components/clusters/managements.vue'
import WarningBar from '@/components/warningBar/warningBar.vue'
import TableBlock from './table.vue'
import { ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'

const form = ref({})
const page = ref(1)
const total = ref(0)
const pageSize = ref(10)
const tableData = ref([])
const title = ref('')
const searchInfo = ref({})
const deleteVisible = ref(false)
const clusters = ref([])

// 搜索
const onReset = () => {
  searchInfo.value = {}
}

// 排序
const handlesortChange = (value) => {
  searchInfo.value = value
  getTableData()
}

// 提交搜索
const onSubmit = () => {
  page.value = 1
  pageSize.value = 10
  getTableData()
}

// 分页
const handleSizeChange = (val) => {
  pageSize.value = val
  getTableData()
}

const handleCurrentChange = (val) => {
  page.value = val
  getTableData()
}

// 查询
const getTableData = async() => {
  const table = await getClustersList({ page: page.value, pageSize: pageSize.value, ...searchInfo.value })
  if (table.code === 0) {
    tableData.value = table.data.list
    total.value = table.data.total
    page.value = table.data.page
    pageSize.value = table.data.pageSize
  }
}

getTableData()

// 更新数据模态框
const dialogFormVisible = ref(false)
const type = ref('')
const handleUpdate = async(row) => {
  const res = await getClustersById({ id: row.id })
  type.value = 'update'
  title.value = '更新'
  if (res.code === 0) {
    form.value = res.data.cluster
    dialogFormVisible.value = true
  }
}

// 关闭模态框
const closeDialog = () => {
  dialogFormVisible.value = false
  dialogRbacFormVisible.value = false
  form.value = {}
}

// 删除数据
const handleDelete = async(row) => {
  ElMessageBox.confirm('您确定要删除  ' + row.name + ' 该集群？, 集群相关授权也会一并删除，是否继续?', '提示', {
    confirmButtonText: '确定',
    cancelButtonText: '取消',
    type: 'warning'
  })
    .then(async() => {
      row.visible = false
      const res = await DeleteCluster({ id: row.id })
      if (res.code === 0) {
        ElMessage({
          type: 'success',
          message: '删除成功'
        })
        if (tableData.value.length === 1 && page.value > 1) {
          page.value--
        }
        getTableData()
      }
    })
    .catch(() => {
      ElMessage({
        type: 'info',
        message: '已取消删除!'
      })
    })
}

// 批量操作
const handleSelectionChange = (val) => {
  clusters.value = val
}

// 批量删除
const onDelete = async() => {
  const ids = clusters.value.map(item => item.id)
  if (ids.length === 0) {
    ElMessage({
      message: '请选择要删除的集群！',
      type: 'warning',
    })
    return
  }
  ElMessageBox.confirm('您确定要删除选中的集群？, 集群相关授权也会一并删除，是否继续?', '提示', {
    confirmButtonText: '确定',
    cancelButtonText: '取消',
    type: 'warning'
  })
    .then(async() => {
      const res = await DeleteClusterByIds({ ids })
      if (res.code === 0) {
        ElMessage({
          type: 'success',
          message: res.msg
        })
        if (tableData.value.length === ids.length && page.value > 1) {
          page.value--
        }
        deleteVisible.value = false
        getTableData()
      }
    })
    .catch(() => {
      ElMessage({
        type: 'info',
        message: '已取消删除!'
      })
    })
}

// 提交数据提示信息
const handleOptions = async(res) => {
  if (res.code === 0) {
    ElMessage({
      type: 'success',
      message: res.msg
    })
    if (tableData.value.length === 1 && page.value > 1) {
      page.value--
    }
    getTableData()
  }
}

// 提交数据
const enterDialog = async(value) => {
  let res
  switch (type.value) {
    case 'create':
      res = await CreateCluster(value)
      break
    case 'update':
      res = await UpdateCluster(value)
      break
    default:
      res = await CreateCluster(value)
      break
  }

  if (res.code === 0) {
    closeDialog()
    handleOptions(res)
  }
}

// 创建
const openDialog = () => {
  type.value = 'create'
  title.value = '创建'
  form.value = {}
  dialogFormVisible.value = true
}

// 授权
const dialogRbacFormVisible = ref(false)
const handleRbacManage = async(row) => {
  title.value = '集群授权'
  form.value = row
  dialogRbacFormVisible.value = true
}

// 关联集群
const dialogManagementsFormVisible = ref(false)
const handleAssociat = (row) => {
  title.value = '集群关联'
  form.value = row
  dialogManagementsFormVisible.value = true
}
</script>
<style></style>
