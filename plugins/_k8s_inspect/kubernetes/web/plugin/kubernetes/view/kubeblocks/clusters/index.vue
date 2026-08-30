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
          </el-form-item>
        </el-form>
        <el-form-item>
          <div class="flex flex-wrap items-center">
            <el-dropdown>
              <el-button icon="plus" size="small" type="primary">添加</el-button>
              <template #dropdown>
                <el-dropdown-menu>
                  <div v-for="index in definitionTypes" :key="index">
                    <el-dropdown-item icon="Coin" @click="openDialog(index)">{{ index }}</el-dropdown-item>
                  </div>
                </el-dropdown-menu>
              </template>
            </el-dropdown>
          </div>
        </el-form-item>
      </div>
      <div v-if="tableData">
        <TableBlock :table-data="tableData" @delete="handleDelete" @detail="handleDetail" @update="handleUpdate" />
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
      <el-dialog v-model="dialogFormVisible" width="60%" :title="title">
        <template #default>
          <div class="dialog-footer">
            <FormBlock :cluster_id="searchInfo.cluster_id" :form="form" :type="type" :versions="versions" @close="closeDialog" @enter="enterDialog" />
          </div>
        </template>
      </el-dialog>
    </div>
    <div v-if="dialogDetailVisible">
      <el-dialog v-model="dialogDetailVisible" width="80%" :title="title">
        <template #default>
          <div class="dialog-footer">
            <DetailBlock :data="form" @close="closeDialog" @enter="enterDialog" />
          </div>
        </template>
      </el-dialog>
    </div>
  </div>
</template>

<script>

export default {
  name: 'Clustes',
}
</script>

<script setup>
import { getK8sObject } from '@/plugin/kubernetes/utils/k8s'
import FormBlock from './form.vue'
import TableBlock from './table.vue'
import DetailBlock from './detail.vue'
import { ref } from 'vue'
import { ElMessage } from 'element-plus'
import {
  ClustersCreate,
  ClustersDelete,
  ClustersGet,
  ClustersList,
  ClustersUpdate,
} from '@/plugin/kubernetes/api/kubeblocks/clusters'
import { ClusterDefinitionsList } from '@/plugin/kubernetes/api/kubeblocks/clusterdefinitions'
import {ClusterVersionsGet, ClusterVersionsList} from '@/plugin/kubernetes/api/kubeblocks/clusterversions'
const form = ref({})
const page = ref(1)
const total = ref(0)
const pageSize = ref(10)
const tableData = ref([])
const title = ref('')
const searchInfo = ref({ 'keywords': '', 'namespace': '' })

// 搜索重置
const onReset = () => {
  searchInfo.value.keywords = ''
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
const definitionTypes = ref([])
const getTableData = async() => {
  const table = await ClustersList(searchInfo.value.cluster_id, page.value, pageSize.value, searchInfo.value.namespace, searchInfo.value.keywords, '')
  if (table.code === 0) {
    tableData.value = table.data.items
    total.value = table.data.total
    page.value = table.data.page
    pageSize.value = table.data.pageSize
  }

  definitionTypes.value = await definitionsList()
}

// 更新数据模态框
const dialogEditVisible = ref(false)
const type = ref('')
const handleUpdate = async(row) => {
  type.value = 'update'
  title.value = '编辑'
  const res = await ClustersGet(searchInfo.value.cluster_id, row.metadata.namespace, row.metadata.name)
  if (res.data.items) {
    if (res.data.items.code) {
      ElMessage({
        type: 'error',
        message: res.data.items.message,
      })
      return
    }
  }

  if (res.code === 0) {
    form.value = res.data.items
    dialogFormVisible.value = true
  }
}

// 关闭模态框
const closeDialog = () => {
  dialogFormVisible.value = false
  form.value = {}
}

// 删除数据
const handleDelete = async(row) => {
  row.visible = false
  const res = await ClustersDelete(searchInfo.value.cluster_id, row.metadata.namespace, row.metadata.name)
  if (res.data.items) {
    if (res.data.items.code) {
      ElMessage({
        type: 'error',
        message: res.data.items.message,
      })
      return
    }
  }

  if (res.code === 0) {
    ElMessage({
      type: 'success',
      message: '删除成功'
    })
    getTableData()
  }
}

// 提交数据
const enterDialog = async(row) => {
  const data = JSON.parse(JSON.stringify(row))
  const ret = await ClusterVersionsGet(searchInfo.value.cluster_id, data.spec.clusterVersionRef)
  if (ret.data.items.code) {
    ElMessage({
      type: 'error',
      message: ret.data.items.message,
    })
    return
  }

  if (ret.code === 0) {
    data.spec.clusterDefinitionRef = ret.data.items.spec.clusterDefinitionRef
    data.spec.componentSpecs.forEach(item => {
      item.replicas = parseInt(item.replicas)
      item.resources.limits.memory = item.resources.limits.memory.toString() + 'Gi'
    })
  }

  let res
  switch (type.value) {
    case 'create':
      res = await ClustersCreate(searchInfo.value.cluster_id, data.metadata.namespace, data)
      break
    case 'update':
      res = await ClustersUpdate(searchInfo.value.cluster_id, data.metadata.namespace, data.metadata.name, data)
      break
    default:
      res = await ClustersCreate(searchInfo.value.cluster_id, data)
      break
  }

  if (res.data.items) {
    if (res.data.items.code) {
      ElMessage({
        type: 'error',
        message: res.data.items.message,
      })
      return
    }
  }

  if (res.code === 0) {
    closeDialog()
  }
}

const definitionsList = async() => {
  const definitionTypes = ref([])
  const res = await ClusterDefinitionsList(searchInfo.value.cluster_id)
  if (res.code === 0) {
    if (res.data.items) {
      res.data.items.forEach(function(clusterDefinition) {
        definitionTypes.value.push(clusterDefinition.spec.type)
      })
    }

    return [...new Set(definitionTypes.value)]
  }
}

// 选择类型
const versions = ref([])
const TypeChange = async(value) => {
  versions.value = []
  const res = await ClusterVersionsList(searchInfo.value.cluster_id)
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

// 创建
const dialogFormVisible = ref(false)
const openDialog = async(value) => {
  console.log(value)
  type.value = 'create'
  title.value = '创建'
  await TypeChange(value)
  form.value = await getK8sObject(value, searchInfo.value.namespace, '')
  if (Object.keys(form.value).length !== 0) {
    dialogFormVisible.value = true
  }
}

// 详情
const dialogDetailVisible = ref(false)
const handleDetail = async(row) => {
  const res = await ClustersGet(searchInfo.value.cluster_id, row.metadata.namespace, row.metadata.name)
  if (res.data.items) {
    if (res.data.items.code) {
      ElMessage({
        type: 'error',
        message: res.data.items.message,
      })
      return
    }
  }

  if (res.code === 0) {
    title.value = '详情'
    form.value = res.data.items
    form.value['cluster_id'] = searchInfo.value.cluster_id
    dialogDetailVisible.value = true
  }
}

window.addEventListener('setItemCluster', (e) => {
  searchInfo.value.cluster_id = parseInt(localStorage.getItem('cluster_id'))
})

window.addEventListener('setItemNamespace', (e) => {
  searchInfo.value.namespace = localStorage.getItem('namespace')
  getTableData()
})
</script>
<style>
</style>
