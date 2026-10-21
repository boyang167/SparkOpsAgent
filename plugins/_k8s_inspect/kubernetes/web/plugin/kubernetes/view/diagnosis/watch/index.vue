<template>
  <div>
    <div class="gva-table-box">
      <div style="overflow: hidden;width: 100%;">
        <el-form
            ref="searchForm"
            :inline="true"
            :rules="rules"
            :model="watchData"
        >
          <el-form-item
              label="类名"
              prop="className"
          >
            <el-input v-model="watchData.className" @keyup.enter="handleSearch"  autocomplete="off" />
          </el-form-item>

          <el-form-item
              label="方法名"
              prop="methodName"
          >
            <el-input v-model="watchData.methodName" @keyup.enter="handleSearch" autocomplete="off" />
          </el-form-item>
          <el-form-item label-width="100px">
            <el-button size="small" type="primary" icon="refresh"   @click="handleSearch">
              查询
            </el-button>
          </el-form-item>
        </el-form>
      </div>
      <TableBlock :table-data="TableData" :cluster_id="searchInfo.cluster_id" />
    </div>
  </div>
</template>

<script>

export default {
  name: 'watch',
}
</script>

<script setup>
import { ref } from 'vue'
import TableBlock from './table.vue'
import {diagnosisProxy} from "@/plugin/kubernetes/api/diagnosis";

const searchInfo = ref({ 'keywords': '', 'namespace': '', 'containerName': '', 'podName': ''})
const watchData =  ref({"className": '', "methodName": '', "execNumber": 30})
const TableData = ref([])

const rules = ref({
  className: [{ required: true, message: '请输入类名', trigger: 'blur' }],
  methodName: [{ required: true, message: '请输入方法名', trigger: 'blur' }],
})


// 异步获取仪表板数据
const watchStatus = ref(false)
const watchsyncExec = async() => {
  if (searchInfo.value.containerName && searchInfo.value.podName) {
    searchInfo.value['body'] = {
      "action": "async_exec",
      "command": "watch " + watchData.value.className + " "  + watchData.value.methodName,
      "sessionId" : sessionId.value
    }

    const ExecData = await diagnosisProxy(searchInfo.value)
    if (ExecData.code === 0) {
      if (ExecData.data.arthas) {
        if (ExecData.data.arthas.body?.jobStatus === "READY") {
          watchStatus.value = true
        } else {
          watchStatus.value = false
        }
      } else {
        watchStatus.value = false
      }
    }
  }
}

// 初始化session
const consumerId = ref('')
const sessionId = ref('')
const initSession = async() => {
  if (searchInfo.value.containerName && searchInfo.value.podName) {
    searchInfo.value['body'] = {
      "action": "init_session"
    }
    const responseData = await diagnosisProxy(searchInfo.value)
    if (responseData.code === 0) {
      if (responseData.data.arthas) {
        consumerId.value = responseData.data.arthas.consumerId
        sessionId.value = responseData.data.arthas.sessionId
      }
    }
  }
}

// 初始化数据
const watchHtmlData  = ref('')
const InitSearchData = async() => {
  if (!watchStatus.value) {
    watchHtmlData.value = ''
    await initSession()
    await watchsyncExec()
  }

  if (searchInfo.value.containerName && searchInfo.value.podName && watchStatus.value) {
    searchInfo.value['body'] = {
      "action":"pull_results",
      "sessionId" : sessionId.value,
      "consumerId": consumerId.value
    }

    const responseData = await diagnosisProxy(searchInfo.value)
    if (responseData.code === 0) {
      if (responseData.data.arthas) {
        if (responseData.data.arthas.state === "SUCCEEDED") {
          responseData.data.arthas.body.results.forEach(result => {
            if ('accessPoint' in result) {
              TableData.value.unshift(result)
            }
          });
        } else {
          watchStatus.value = false
        }
      }
    }
  }
}

initSession()

setInterval(() => {
  if (watchStatus.value) {
    InitSearchData();
  }
}, 3000);


// 查询
const handleSearch = async() => {
  watchStatus.value = false
  TableData.value = []
  await InitSearchData()
}

// 事件监听
window.addEventListener('setItemCluster', (e) => {
  searchInfo.value.cluster_id = parseInt(localStorage.getItem('cluster_id'))
})

window.addEventListener('setItemNamespace', (e) => {
  searchInfo.value.namespace = localStorage.getItem('namespace')
})

window.addEventListener('setItempodName', (e) => {
  searchInfo.value.podName = localStorage.getItem('podName')
})

window.addEventListener('setItemcontainerName', (e) => {
  searchInfo.value.containerName = localStorage.getItem('containerName')
  initSession()
})
</script>
<style>
.button-container {
  display: flex;              /* 使用 Flexbox */
  justify-content: center;    /* 水平居中 */
  align-items: center;        /* 垂直居中（如果有需要的话） */
}
</style>
