<template>
  <div class="gva-search-box">
    <el-form
      ref="searchForm"
      :inline="true"
      :model="searchInfo"
    >
      <el-form-item label="集群">
        <el-select
          v-model="searchInfo.cluster_id"
          style="width: 150px"
          placeholder="请选择集群"
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
      <el-form-item
        v-show="namespaceStatus"
        label="命名空间"
      >
        <el-select
          v-model="searchInfo.namespace"
          filterable
          style="width: 150px"
          placeholder="请选择命名空间"
          @change="NamespaceChange"
        >
          <el-option
            v-for="item in namespace_list"
            :key="item.id"
            :label="item.name"
            :value="item.name"
          />
        </el-select>
      </el-form-item>
      <el-form-item
        label="Pod"
      >
        <el-select
          v-model="searchInfo.podName"
          filterable
          style="width: 150px"
          placeholder="请选择Pod"
          @change="PodChange"
        >
          <el-option
            v-for="item in pods_list"
            :key="item.id"
            :label="item.name"
            :value="item.name"
          />
        </el-select>
      </el-form-item>
      <el-form-item
        label="Container"
      >
        <el-select
          v-model="searchInfo.containerName"
          style="width: 150px"
          placeholder="请选择要安装的Container"
          @change="ContainerChange"
        >
          <el-option
            v-for="item in containers_list"
            :key="item.id"
            :label="item.name"
            :value="item.name"
          />
        </el-select>
      </el-form-item>
      <el-form-item
          label="进程"
          v-show="diagnosisData.id === 0 || !diagnosisData.id"
      >
        <el-select
            v-model="searchInfo.processId"
            style="width: 150px"
            placeholder="请选择要安装的Container"
        >
          <el-option
              v-for="item in process_list"
              :key="item.value"
              :label="item.name"
              :value="item.value"
          />
        </el-select>
      </el-form-item>
      <el-form-item>
        <el-button
          v-show="diagnosisData.id === 0 || !diagnosisData.id"
          type="primary"
          size="small"
          icon="SetUp"
          @click="handleInstall"
        >
          安装
        </el-button>
      </el-form-item>
      <el-form-item v-if="diagnosisData.id">
        <el-button
          v-show="diagnosisData.id !== 0"
          type="danger"
          size="small"
          icon="Delete"
          @click="handleUninstall"
        >
          卸载
        </el-button>
        <el-button
          v-show="diagnosisData.id !== 0"
          type="primary"
          size="small"
          icon="Expand"
          @click="handleTerminal"
        >
          终端
        </el-button>
      </el-form-item>
    </el-form>

    <div v-if="dialogTerminalVisible">
      <el-dialog
        v-model="dialogTerminalVisible"
        width="60%"
        :show-close="false"
        :close-on-press-escape="false"
        :close-on-click-modal="false"
        :fullscreen="dialogFull"
        :title="title"
        :center="true"
      >
        <template #default>
          <div style="text-align:right;">
            <el-button type="primary" link icon="FullScreen" @click="handledialogFull" />
            <el-button type="primary" link icon="Close" @click="closeDialog" />
          </div>
          <div class="dialog-footer">
            <TerminalBlock :data="searchInfo" @close="closeDialog" />
          </div>
        </template>
      </el-dialog>
    </div>
  </div>
</template>

<script>
export default {
  name: 'ClusterNamespacePods',
  components: {

  }
}
</script>

<script setup>
import { ref } from 'vue'
import TerminalBlock from '@/plugin/kubernetes/components/diagnosis/select/terminal.vue'
import { getClustersList } from '@/plugin/kubernetes/api/clusters'
import { getClusterUserNamespace } from '@/plugin/kubernetes/api/namespaces'
import {
  dispatchClusterIDEventStrage,
  dispatchContainerNameEventStrage,
  dispatchNamespaceEventStrage, dispatchPodNameEventStrage
} from '@/plugin/kubernetes/utils/watchLocalStorage'
import {PodsGet, PodsList} from "@/plugin/kubernetes/api/pods";
import {diagnosisGetByPod, diagnosisInstall, diagnosisJps, diagnosisUninstall} from "@/plugin/kubernetes/api/diagnosis";
import {ElMessage} from "element-plus";

const props = defineProps({
  namespace: {
    default: function() {
      return true
    },
    type: Boolean
  },
})

const dialogFull = ref(false)
const dialogTerminalVisible = ref(false)
const searchInfo = ref({ 'keywords': '', 'cluster_id': 0, 'namespace': '', 'containerName': '', 'podName': '', "processId": ''})
const namespace_list = ref([])
const namespaceStatus = ref(true)
const process_list = ref([])

// 全屏
const handledialogFull = () => {
  if (dialogFull.value) {
    dialogFull.value = false
  } else {
    dialogFull.value = true
  }
}


// 关闭模态框
const closeDialog = () => {
  dialogTerminalVisible.value = false
  dialogFull.value = false
}

// 切换命名空间
const NamespaceChange = async(value) => {
  searchInfo.value['namespace'] = value
  searchInfo.value['podName'] = ''
  dispatchNamespaceEventStrage(searchInfo.value.namespace)
  await getNamespacePodsData()
}

// 集群选择
const ClusterChange = async(value) => {
  dispatchClusterIDEventStrage(searchInfo.value.cluster_id)
  await getNamespaceData(searchInfo.value.cluster_id)
  searchInfo.value['cluster_id'] = value
}

const defaultNamespace = ref({})
const getNamespaceData = async(cluster_id) => {
  const localStorageHit = ref(false)
  defaultNamespace.value = { 'id': 0, 'name': 'All Namespaces' }
  const nstable = await getClusterUserNamespace({ 'id': cluster_id })
  if (nstable.code === 0) {
    namespace_list.value = []
    for (const ns of nstable.data.namespaces) {
      if (ns === localStorage.getItem('namespace')) {
        localStorageHit.value = true
      }

      const item = {
        id: Math.random(),
        name: ns,
      }
      namespace_list.value.push(item)
    }

    namespace_list.value.push(defaultNamespace.value)

    if (localStorage.getItem('namespace') === 'All Namespaces') {
      localStorageHit.value = true
    }

    if (localStorageHit.value) {
      searchInfo.value.namespace = localStorage.getItem('namespace')
    } else {
      searchInfo.value.namespace = namespace_list.value[0].name
    }

    dispatchNamespaceEventStrage(searchInfo.value.namespace)
    await getNamespacePodsData()
  }
}

const cluster_list = ref([])
const getClusterData = async() => {
  const table = await getClustersList({ page: 1, pageSize: 10000 })
  if (table.code === 0) {
    cluster_list.value = table.data.list
    searchInfo.value.cluster_id = cluster_list.value[0].id
    if (parseInt(localStorage.getItem('cluster_id')) > 0) {
      searchInfo.value.cluster_id = parseInt(localStorage.getItem('cluster_id'))
    }
    dispatchClusterIDEventStrage(searchInfo.value.cluster_id)
    await getNamespaceData(searchInfo.value.cluster_id)
  }
}

getClusterData()

// 数据初始化
const FormInitData = () => {
  namespaceStatus.value = props.namespace
}

FormInitData()

// 安装
const handleInstall = async() => {
  const res = await diagnosisInstall(searchInfo.value)
  if (res.code === 0) {
    ElMessage({
      type: 'success',
      message: '安装成功'
    })

    fetchDiagnosisData()
  }
}

// 卸载
const handleUninstall = async() => {
  const res = await diagnosisUninstall(searchInfo.value)
  if (res.code === 0) {
    ElMessage({
      type: 'success',
      message: '卸载成功'
    })
  }
}

// 选中Pod
const PodChange =  async(value) => {
  searchInfo.value['podName'] = value
  dispatchPodNameEventStrage(value)
  await getPodsContainersData()
}

// 获取已创建转发的Pod信息
const diagnosisData = ref({})
const fetchDiagnosisData = async () => {
  if (searchInfo.value.containerName && searchInfo.value.podName) {
    const res = await diagnosisGetByPod(searchInfo.value)
    if (res.code === 0) {
      diagnosisData.value = res.data.diagnosis
    }
  }
};

const fetchDiagnosisJpsData = async () => {
  if (searchInfo.value.containerName && searchInfo.value.podName) {
    const res = await diagnosisJps(searchInfo.value)
    if (res.code === 0) {
      process_list.value = res.data.diagnosis
    }
  }
};

// 选中 Container
const ContainerChange = async(value) => {
  dispatchContainerNameEventStrage(value)
  await fetchDiagnosisJpsData()
  await fetchDiagnosisData()
}

// 获取命名空间Pods
const pods_list = ref([])
const getNamespacePodsData = async() => {
  pods_list.value = []
  const podstable = await PodsList(searchInfo.value.cluster_id, 0, 0, searchInfo.value.namespace, "", '', '')
  if (podstable.code === 0) {
    for (const pod of podstable.data.items) {
      const item = {
        id: Math.random(),
        name: pod.metadata.name,
      }
      pods_list.value.push(item)
    }
  }
}

// 获取Containers
const containers_list = ref([])
const getPodsContainersData = async() => {
  containers_list.value = []
  const podtable = await PodsGet(searchInfo.value.cluster_id, searchInfo.value.namespace, searchInfo.value.podName)
  if (podtable.code === 0) {
    if (podtable.data.items) {
      for (const container of podtable.data.items.spec.containers) {
        const item = {
          id: Math.random(),
          name: container.name,
        }
        containers_list.value.push(item)
      }
    }

    await fetchDiagnosisData()
  }
}


// 终端
const title =  ref('')
const handleTerminal = () => {
  if (searchInfo.value.containerName && searchInfo.value.podName) {
    title.value = '容器组名：' + searchInfo.value.podName + ' 容器名：' + searchInfo.value.containerName + ' 终端'
    dialogTerminalVisible.value = true
  }
}

fetchDiagnosisData()

// 每隔5秒调用一次 fetchDiagnosisData 函数
setInterval(() => fetchDiagnosisData(), 3000);

</script>
<style scoped>

</style>
