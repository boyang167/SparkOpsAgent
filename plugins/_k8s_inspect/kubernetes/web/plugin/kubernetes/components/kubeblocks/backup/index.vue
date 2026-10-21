<template>
  <div>
    <div class="gva-btn-list">
      <el-button icon="plus" link size="small" type="primary" @click="openDialog('addApi')">备份</el-button>
      <el-button icon="refresh" link size="small"   type="primary" @click="onReset">刷新</el-button>
    </div>
    <div v-if="tableData">
      <TableBlock :table-data="tableData" @delete="handleDelete" @detail="handleDetail" @update="handleUpdate" />
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
      <el-dialog v-model="dialogYamlVisible" :title="title" width="60%">
        <template #default>
          <div class="dialog-footer">
            <YamlBlock :data="form" :read-only="readOnly" @close="closeDialog" @enter="enterDialog" />
          </div>
        </template>
      </el-dialog>
    </div>
  </div>
</template>

<script>
export default {
  name: 'Backup'
}
</script>
<script setup>
import { ref } from 'vue'
import { backupsCreate, backupsDelete, backupsList, backupsUpdate } from '@/plugin/kubernetes/api/kubeblocks/backups'
import YamlBlock from '@/plugin/kubernetes/components/yaml/index.vue'
import TableBlock from './table.vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { getK8sObject } from '@/plugin/kubernetes/utils/k8s'
import { IngressesCreate, IngressesUpdate } from '@/plugin/kubernetes/api/ingress'
const emit = defineEmits(['close', 'enter'])
const props = defineProps({
  cluster_id: {
    default: function() {
      return 0
    },
    type: Number
  },
  name: {
    default: function() {
      return ''
    },
    type: String
  },
  namespace: {
    default: function() {
      return ''
    },
    type: String
  },
  selector: {
    default: function() {
      return ''
    },
    type: String
  }
})

// 查询
const page = ref(1)
const total = ref(0)
const pageSize = ref(10)
const tableData = ref([])
const getTableData = async() => {
  const table = await backupsList(cluster_id.value, namespace.value, selector.value, page.value, pageSize.value)
  if (table.code === 0) {
    tableData.value = table.data.items
    total.value = table.data.total
    page.value = table.data.page
    pageSize.value = table.data.pageSize
  }
}

const name = ref('')
const namespace = ref('')
const selector = ref('')
const cluster_id = ref(0)
const dialogYamlVisible = ref(false)
const readOnly = ref(false)
const title = ref('')
const form = ref({})
const type = ref('')
const FormInitData = () => {
  name.value = props.name
  namespace.value = props.namespace
  cluster_id.value = props.cluster_id
  selector.value = props.selector
}

FormInitData()

getTableData()

// 分页
const handleSizeChange = (val) => {
  pageSize.value = val
  getTableData()
}

const handleCurrentChange = (val) => {
  page.value = val
  getTableData()
}

// 更新
const handleUpdate = (val) => {
  title.value = '更新'
  type.value = 'update'
  form.value = val
  readOnly.value = false
  dialogYamlVisible.value = true
}

// 删除
const handleDelete = (row) => {
  ElMessageBox.confirm('您确定要删除  ' + row.metadata.name + ' 该备份？, 是否继续?', '提示', {
    confirmButtonText: '确定',
    cancelButtonText: '取消',
    type: 'warning'
  })
    .then(async() => {
      const res = await backupsDelete(cluster_id.value, row.metadata.namespace, row.metadata.name)
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
    })
    .catch(() => {
      ElMessage({
        type: 'info',
        message: '已取消删除!'
      })
    })
}

// 详情
const handleDetail = (val) => {
  title.value = '详情'
  form.value = val
  readOnly.value = true
  dialogYamlVisible.value = true
}

// 关闭模态框
const closeDialog = () => {
  dialogYamlVisible.value = false
  form.value = {}
}

// 确定
const enterDialog = async(row) => {
  let res
  switch (type.value) {
    case 'create':
      res = await backupsCreate(cluster_id.value, row.metadata.namespace, row)
      break
    case 'update':
      res = await backupsUpdate(cluster_id.value, row.metadata.namespace, row.metadata.name, row)
      break
    default:
      res = await backupsCreate(cluster_id.value, row)
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

// 新增备份
const openDialog = async() => {
  type.value = 'create'
  title.value = '新增'
  form.value = await getK8sObject('backupMysql', namespace.value, name.value)
  readOnly.value = false
  dialogYamlVisible.value = true
}

// 刷新列表
const onReset = () => {
  getTableData()
}
</script>
<style scoped>

</style>
