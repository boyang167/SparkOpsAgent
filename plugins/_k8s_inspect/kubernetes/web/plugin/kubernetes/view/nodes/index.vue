<template>
  <div>
    <div class="gva-search-box">
      <el-form
        ref="searchForm"
        :inline="true"
        :model="searchInfo"
      >
        <el-form-item label="集群">
          <el-select
            v-model="searchInfo.cluster_id"
            placeholder="请选择集群"
            style="width: 200px"
            @change="ClusterChange"
          >
            <el-option
              v-for="item in cluster_list"
              :key="item.id"
              :label="item.name"
              :value="item.id"
            />
          </el-select>
        </el-form-item>
        <el-form-item label="名称">
          <el-input
            v-model="searchInfo.keywords"
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
          <el-button
            size="small"
            type="primary"
            icon="star"
            @click="ToNodeLabelsTaint"
          >标签与污点</el-button>
        </el-form-item>
        <el-form-item>
          <el-button
            size="small"
            type="primary"
            icon="Cloudy"
            @click="handleCloudTty"
          >CloudShell</el-button>
        </el-form-item>
        <el-form-item>
          <el-button
              size="small"
              type="primary"
              icon="ai-gva"
              @click="handleChat"
          >Ai 助手</el-button>
        </el-form-item>
      </el-form>
    </div>
    <div class="gva-table-box">
      <div v-if="tableData">
        <TableBlock
          :table-data="tableData"
          @update="handleUpdate"
          @delete="handleDelete"
          @schedule="handleSchedule"
          @drain="handleDrain"
          @detail="handleDetail"
          @yaml="handleYAML"
          @monitor="handleMonitor"
          @shell="handleShell"
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
        <template #footer>
          <div class="dialog-footer">
            <FormBlock
              :form="form"
              :optype="type"
              @close="closeDialog"
              @enter="enterDialog"
            />
          </div>
        </template>
      </el-dialog>
    </div>
    <div v-if="dialogDetailVisible">
      <el-dialog
        v-model="dialogDetailVisible"
        width="70%"
        :title="title"
      >
        <DetailBlock :form="form" />
      </el-dialog>
    </div>
    <div v-if="dialogYamlVisible">
      <el-dialog
        v-model="dialogYamlVisible"
        width="60%"
        :title="title"
      >
        <template #default>
          <div class="dialog-footer">
            <YamlBlock
              :data="form"
              @close="closeDialog"
              @enter="enterYamlDialog"
            />
          </div>
        </template>
      </el-dialog>
    </div>
    <div v-if="dialogMonitorVisible">
      <el-dialog
        ref="monitor"
        v-model="dialogMonitorVisible"
        width="60%"
        :title="title"
      >
        <template #default>
          <div class="dialog-footer">
            <MonitorBlock
              :data="form"
              @close="closeDialog"
            />
          </div>
        </template>
      </el-dialog>
    </div>

    <div v-if="dialogCloudTtyVisible">
      <el-dialog
        v-model="dialogCloudTtyVisible"
        width="70%"
        :title="title"
        :center="true"
      >
        <template #default>
          <div class="dialog-footer">
            <CloudTTYBlock
              @close="closeDialog"
            />
          </div>
        </template>
      </el-dialog>
    </div>

    <div v-if="dialogAiChatVisible">
      <el-dialog
          v-model="dialogAiChatVisible"
          width="70%"
          :title="title"
          top="3vh"
      >
        <template #default>
          <div class="dialog-footer">
            <ChatBlock
                @close="closeDialog"
            />
          </div>
        </template>
      </el-dialog>
    </div>

    <div v-if="dialogNodeTtyVisible">
      <el-dialog
        v-model="dialogNodeTtyVisible"
        width="70%"
        :title="title"
        :center="true"
        @close="closeNodeTTyDialog"
      >
        <template #default>
          <div class="dialog-footer">
            <NodeTTYBlock
              :node_name="node_name"
            />
          </div>
        </template>
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
} from '@/plugin/kubernetes/api/clusters'
import {
  NodesList,
  NodesGet,
  NodesUpdate,
  NodesDelete
} from '@/plugin/kubernetes/api/nodes'
// 移除节点指标数据获取，不再依赖 NodeMetricsList
import {
  PodsList,
  Podseviction, PodsDelete,
} from '@/plugin/kubernetes/api/pods'
import { cpuUnitConvert, memoryUnitConvert } from '@/plugin/kubernetes/utils/unitConvert'
import FormBlock from './form.vue'
import TableBlock from './table.vue'
import DetailBlock from './detail.vue'
import MonitorBlock from './monitor.vue'
import YamlBlock from '@/plugin/kubernetes/components/yaml/index.vue'
import CloudTTYBlock from '@/plugin/kubernetes/components/cloudtty/index.vue'
import ChatBlock from '@/plugin/kubernetes/components/chat/index.vue'
import NodeTTYBlock from '@/plugin/kubernetes/components/nodetty/index.vue'
import { ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { useRouter } from 'vue-router'
import { dispatchClusterIDEventStrage } from '@/plugin/kubernetes/utils/watchLocalStorage'
import { useUserStore } from '@/pinia/modules/user'

const form = ref({
  apiVersion: 'v1',
  kind: 'Node',
  metadata: {
    name: '',
    annotations: {},
    labels: {},
  },
})
const page = ref(1)
const total = ref(0)
const pageSize = ref(10)
const tableData = ref([])
const title = ref('')
const searchInfo = ref({ 'keywords': '' })
const cluster_list = ref([])
const router = useRouter()
// 搜索
const onReset = () => {
  searchInfo.value = {}
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
const getClusterData = async() => {
  const table = await getClustersList({ page: 1, pageSize: 10000 })
  if (table.code === 0) {
    cluster_list.value = table.data.list
    searchInfo.value.cluster_id = cluster_list.value[0].id
    if (parseInt(localStorage.getItem('cluster_id')) > 0) {
      searchInfo.value.cluster_id = parseInt(localStorage.getItem('cluster_id'))
      dispatchClusterIDEventStrage(searchInfo.value.cluster_id)
    } else {
      dispatchClusterIDEventStrage(cluster_list.value[0].id)
    }
  }
}

// 查询
const getTableData = async(status) => {
  if (status) {
    await getClusterData()
  }
  const table = await NodesList(searchInfo.value.cluster_id, page.value, pageSize.value, searchInfo.value.keywords)
  if (table.code === 0) {
    // 聚合节点上所有 Pod 的 CPU/内存 Requests 与 Limits
    for (const node of table.data.items) {
      try {
        const podsResp = await PodsList(
          searchInfo.value.cluster_id,
          '',
          '',
          '',
          '',
          '',
          `spec.nodeName=${node.metadata.name}`
        )
        if (podsResp.code === 0 && podsResp.data?.items) {
          let cpuReq = 0
          let cpuLim = 0
          let memReq = 0
          let memLim = 0

          for (const pod of podsResp.data.items) {
            const containers = [
              ...(pod.spec?.containers || []),
              ...(pod.spec?.initContainers || []),
            ]
            for (const c of containers) {
              const r = c.resources || {}
              if (r.requests?.cpu) cpuReq += cpuUnitConvert(r.requests.cpu)
              if (r.limits?.cpu) cpuLim += cpuUnitConvert(r.limits.cpu)
              if (r.requests?.memory) memReq += memoryUnitConvert(r.requests.memory)
              if (r.limits?.memory) memLim += memoryUnitConvert(r.limits.memory)
            }
          }

          node.cpuRequests = `${cpuReq}m`
          node.cpuLimits = `${cpuLim}m`
          node.memoryRequests = `${memReq}Mi`
          node.memoryLimits = `${memLim}Mi`

          const cpuAlloc = node.status?.allocatable?.cpu ? cpuUnitConvert(node.status.allocatable.cpu) : 0
          const memAlloc = node.status?.allocatable?.memory ? memoryUnitConvert(node.status.allocatable.memory) : 0
          node.cpuRequestsPersent = cpuAlloc > 0 ? Number(((cpuReq / cpuAlloc) * 100).toFixed(2)) : 0
          node.cpuLimitsPersent = cpuAlloc > 0 ? Number(((cpuLim / cpuAlloc) * 100).toFixed(2)) : 0
          node.memoryRequestsPersent = memAlloc > 0 ? Number(((memReq / memAlloc) * 100).toFixed(2)) : 0
          node.memoryLimitsPersent = memAlloc > 0 ? Number(((memLim / memAlloc) * 100).toFixed(2)) : 0
        } else {
          node.cpuRequests = '0m'
          node.cpuLimits = '0m'
          node.memoryRequests = '0Mi'
          node.memoryLimits = '0Mi'
          node.cpuRequestsPersent = 0
          node.cpuLimitsPersent = 0
          node.memoryRequestsPersent = 0
          node.memoryLimitsPersent = 0
        }
      } catch (err) {
        node.cpuRequests = '0m'
        node.cpuLimits = '0m'
        node.memoryRequests = '0Mi'
        node.memoryLimits = '0Mi'
        node.cpuRequestsPersent = 0
        node.cpuLimitsPersent = 0
        node.memoryRequestsPersent = 0
        node.memoryLimitsPersent = 0
      }
    }
    tableData.value = table.data.items
    total.value = table.data.total
    page.value = table.data.page
    pageSize.value = table.data.pageSize
  }
}

getTableData(true)

// 更新数据模态框
const dialogFormVisible = ref(false)
const type = ref('')
const handleUpdate = async(row) => {
  const res = await NodesGet(searchInfo.value.cluster_id, row.metadata.name)
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
    type.value = 'update'
    title.value = '编辑'
    form.value = res.data.items
    dialogFormVisible.value = true
  }
}

// 关闭模态框
const closeDialog = () => {
  dialogFormVisible.value = false
  dialogYamlVisible.value = false
  dialogMonitorVisible.value = false
  dialogCloudTtyVisible.value = false
  dialogNodeTtyVisible.value = false
  form.value = {}
  getTableData(true)
}

// 删除数据
const handleDelete = async(row) => {
  ElMessageBox.confirm('您确定要删除  ' + row.metadata.name + ' 节点？, 是否继续?', '提示', {
    confirmButtonText: '确定',
    cancelButtonText: '取消',
    type: 'warning'
  })
    .then(async() => {
      row.visible = false
      const res = await NodesDelete(searchInfo.value.cluster_id, row.metadata.name)
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
const enterDialog = async(row) => {
  let res
  switch (type.value) {
    case 'update':
      res = await NodesUpdate(searchInfo.value.cluster_id, row.metadata.name, row)
      break
    default:
      res = await NodesUpdate(searchInfo.value.cluster_id, row.metadata.name, row)
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
    handleOptions(res)
  }
}

// 集群选择
const ClusterChange = async() => {
  dispatchClusterIDEventStrage(searchInfo.value.cluster_id)
  getTableData()
}

// 节点Pod数据
const podlist = ref([])
const PodsListData = async(row) => {
  const res = await PodsList(searchInfo.value.cluster_id, '', '', '', '', '', `fieldSelector=spec.nodeName=${row.metadata.name}`)
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
    podlist.value = res.data.items
  }
}

const handleDrain = async(row) => {
  ElMessageBox.confirm('此操作将节点: ' + row.metadata.name + ' Pod全部排空，是否继续?',
    'Warning',
    {
      confirmButtonText: '确定',
      cancelButtonText: '取消',
      type: 'warning',
      center: true,
    }
  ).then(async() => {
    await PodsListData(row)
    for (const item of podlist.value) {
      const EvictionPodData = {
        apiVersion: 'policy/v1beta1',
        kind: 'Eviction',
        metadata: {
          name: item.metadata.name,
          namespace: item.metadata.namespace,
          creationTimestamp: null,
        },
        deleteOptions: {},
      }
      await Podseviction(searchInfo.value.cluster_id, item.metadata.name, item.metadata.namespace, EvictionPodData)
    }
    ElMessage({
      type: 'success',
      message: '节点Pod排空成功！',
    })
  }).catch(() => {
    ElMessage({
      type: 'info',
      message: '已取消排空',
    })
    getTableData()
  })
}
// 调度设置
const handleSetSchedule = async(row, msg) => {
  ElMessageBox.confirm('此操作将节点: ' + row.metadata.name + msg + ' 是否继续?',
    'Warning',
    {
      confirmButtonText: '确定',
      cancelButtonText: '取消',
      type: 'warning',
      center: true,
    }
  ).then(async() => {
    const res = await NodesUpdate(searchInfo.value.cluster_id, row.metadata.name, row)
    if (res.data.items) {
      if (res.data.items.code) {
        ElMessage({
          type: 'error',
          message: res.data.items.message,
        })
        getTableData()
        return
      }
    }

    if (res.code === 0) {
      ElMessage({
        type: 'success',
        message: '设置成功',
      })
      getTableData()
    }
  }).catch(() => {
    ElMessage({
      type: 'info',
      message: '已取消设置',
    })
    getTableData()
  })
}

// 调度设置判断
const handleSchedule = async(row) => {
  if (row.spec.unschedulable) {
    row.spec.unschedulable = false
    handleSetSchedule(row, '设置可调度')
  } else {
    row.spec.unschedulable = true
    handleSetSchedule(row, '设置不可调度')
  }
}

// 详情
const dialogDetailVisible = ref(false)
const handleDetail = async(row) => {
  form.value = row
  form.value['cluster_id'] = searchInfo.value.cluster_id
  title.value = '详情'
  dialogDetailVisible.value = true
}

// YAML 编辑
const dialogYamlVisible = ref(false)
const handleYAML = async(row) => {
  title.value = '编辑'
  const res = await NodesGet(searchInfo.value.cluster_id, row.metadata.name)
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
    dialogYamlVisible.value = true
  }
}

// YAML编辑提交
const enterYamlDialog = async(row) => {
  const res = await NodesUpdate(searchInfo.value.cluster_id, row.metadata.name, row)
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
      message: '更新成功'
    })
    closeDialog()
    getTableData()
  }
}

// 标签与污点
const ToNodeLabelsTaint = () => {
  router.push({ name: 'nodelabelstaints' })
}

// 监控
const dialogMonitorVisible = ref(false)
const handleMonitor = (row) => {
  title.value = '节点 名称: ' + row.metadata.name
  form.value = row
  form.value['cluster_id'] = searchInfo.value.cluster_id
  dialogMonitorVisible.value = true
}

// CloudTTY
const dialogCloudTtyVisible = ref(false)
const handleCloudTty = () => {
  title.value = 'CloudShell'
  dialogCloudTtyVisible.value = true
}

// AI
const  dialogAiChatVisible = ref(false)
const handleChat = () => {
  title.value = 'AI 助手'
  dialogAiChatVisible.value = true
}

// Shell
const dialogNodeTtyVisible = ref(false)
const node_name = ref('')
const handleShell = (row) => {
  title.value = 'NodeShell 节点名称：' + row.metadata.name
  node_name.value = row.metadata.name
  dialogNodeTtyVisible.value = true
}

// 删除nodeshell pod
const DeleteNodeShell = async(data) => {
  for (const pod of data) {
    await PodsDelete(searchInfo.value.cluster_id, pod.metadata.namespace, pod.metadata.name)
  }
}

// 获取pod 信息
const PodsData = ref([])
const PodLists = async(namespace, selector) => {
  const res = await PodsList(searchInfo.value.cluster_id, '', '', namespace, '', selector, '')
  if (res.code === 0 && res.data.items) {
    PodsData.value = res.data.items
    if (PodsData.value.length > 0) {
      await DeleteNodeShell(PodsData.value)
    }
  }
}

// 关闭模态框
const userStore = useUserStore()
const labelSelector = ref('')
const closeNodeTTyDialog = async() => {
  dialogNodeTtyVisible.value = false
  labelSelector.value = 'devops/username=' + userStore.userInfo.userName
  await PodLists('default', labelSelector.value)
}
</script>
<style>
</style>
