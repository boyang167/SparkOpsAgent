<template>
  <div>
    <warning-bar title="注：请在Kubernetes集群内安装Kubevela控制器及安装Velaux插件以便于使用该功能，建议Kubevela版本 >= v1.9 。" />
    <div class="gva-btn-list">
      <el-button
        size="small"
        link
        type="primary"
        icon="plus"
        @click="handleAdd"
      >添加</el-button>
      <el-button
        size="small"
        link
        type="primary"
        icon="refresh"
        @click="handleflush"
      >刷新</el-button>
    </div>
    <el-table
      ref="multipleTable"
      :data="tableData"
      style="width: 100%"
      tooltip-effect="dark"
      row-key="ID"
    >
      <el-table-column
        align="left"
        label="名称"
        prop="name"
      />
      <el-table-column
        align="left"
        label="角色"
      >
        <template #default="scope">
          <el-tag
            v-if="scope.row.name ==='local'"
            class="ml-2"
            effect="dark"
          >管控节点</el-tag>
          <el-tag
            v-else
            class="ml-2"
            type="success"
            effect="dark"
          >被控节点</el-tag>
        </template>
      </el-table-column>
      <el-table-column
        align="left"
        label="别名"
        prop="alias"
      />
      <el-table-column
        label="状态"
        prop="status"
      >
        <template #default="scope">
          <el-tag
            v-if="scope.row.status ==='Healthy'"
            type="success"
          >Healthy</el-tag>
          <el-tag
            v-else
            type="danger"
          >{{ scope.row.status }}</el-tag>
        </template>
      </el-table-column>
      <el-table-column
        align="left"
        label="操作"
      >
        <template #default="scope">
          <el-button
            v-if="scope.row.name ==='local'"
            size="small"
            type="primary"
            link
            icon="edit"
            disabled
          >编辑</el-button>
          <el-button
            v-else
            size="small"
            type="primary"
            link
            icon="edit"
            @click="handleUpdate(scope.row)"
          >编辑</el-button>
          <el-button
            v-if="scope.row.name ==='local'"
            type="primary"
            link
            icon="delete"
            size="small"
            disabled
          >删除</el-button>
          <el-button
            v-else
            type="primary"
            link
            icon="delete"
            size="small"
            @click="handleDelete(scope.row)"
          >删除</el-button>
        </template>
      </el-table-column>
    </el-table>
    <div class="gva-pagination">
      <el-pagination
        :current-page="page"
        :page-size="pageSize"
        :page-sizes="[10, 30, 50, 100]"
        :style="{ float: 'right', padding: '20px' }"
        :total="total"
        layout="total, sizes, prev, pager, next, jumper"
        @current-change="handleCurrentChange"
        @size-change="handleSizeChange"
      />
    </div>

    <div v-if="dialogJoinFormVisible">
      <el-dialog
        v-model="dialogJoinFormVisible"
        width="70%"
        :title="title"
        :close-on-press-escape="false"
        :close-on-click-modal="false"
      >
        <JoinFormBlock
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
  name: 'ClusterManagement'
}
</script>
<script setup>
import { ref } from 'vue'
import WarningBar from '@/components/warningBar/warningBar.vue'
import JoinFormBlock from './joinclusterform.vue'
import {
  ClustersList,
  ClustersJoin, ClustersDetail, ClustersUpdate, ClustersDelete,
} from '@/plugin/kubernetes/api/kubevela/clusters'
import { ElMessage, ElMessageBox } from 'element-plus'
const props = defineProps({
  form: {
    default: function() {
      return {}
    },
    type: Object
  },
})

const page = ref(1)
const pageSize = ref(10)
const total = ref(0)
const tableData = ref([])
const cluster_id = ref(0)
const type = ref()
const GetClusterList = async() => {
  const res = await ClustersList(cluster_id.value, page.value, pageSize.value, '')
  if (res.code === 0) {
    tableData.value = res.data.clusters
    total.value = res.data.total
  }
}

// 页码调整
const handleCurrentChange = (row) => {
  page.value = row
  GetClusterList()
}

// 页面数量调整
const handleSizeChange = (row) => {
  pageSize.value = row
  GetClusterList()
}

// 表单数据初始化
const GetFormData = () => {
  cluster_id.value = props.form.id
  GetClusterList()
}

GetFormData()

// 刷新
const handleflush = () => {
  GetFormData()
}

// 更新
const handleUpdate = async(row) => {
  title.value = '集群详情'
  type.value = 'update'
  const res = await ClustersDetail(cluster_id.value, row.name)
  if (res.code === 0) {
    form.value = res.data
    dialogJoinFormVisible.value = true
  }
}

// 删除
const handleDelete = async(row) => {
  ElMessageBox.confirm('您确定要删除  ' + row.name + ' 集群？, 是否继续?', '提示', {
    confirmButtonText: '确定',
    cancelButtonText: '取消',
    type: 'warning'
  })
    .then(async() => {
      const res = await ClustersDelete(cluster_id.value, row.name)
      if (res.code === 0) {
        ElMessage({
          showClose: true,
          message: '操作成功',
          type: 'success'
        })
      }
      GetClusterList()
    })
    .catch(() => {
      ElMessage({
        type: 'info',
        message: '已取消删除!'
      })
    })
}

// 添加
const form = ref({})
const dialogJoinFormVisible = ref(false)
const title = ref('')
const handleAdd = () => {
  title.value = '注册集群'
  type.value = 'create'
  dialogJoinFormVisible.value = true
}

// 关闭模态框
const closeDialog = () => {
  dialogJoinFormVisible.value = false
  form.value = {}
}

// 批量删除
const handleDel = () => {
  console.log('批量删除')
}

// 提交数据
const enterDialog = async(value) => {
  let res
  switch (type.value) {
    case 'create':
      res = await ClustersJoin(cluster_id.value, value)
      break
    case 'update':
      res = await ClustersUpdate(cluster_id.value, value.name, value)
      break
    default:
      res = await ClustersJoin(cluster_id.value, value)
      break
  }

  if (res.data.BusinessCode) {
    ElMessage({
      showClose: true,
      message: '操作失败：' + res.data.Message,
      type: 'error'
    })
    return
  }

  if (res.code === 0) {
    dialogJoinFormVisible.value = false
    GetClusterList()
    ElMessage({
      showClose: true,
      message: '操作成功',
      type: 'success'
    })
  }
}
</script>
<style scoped>
</style>
