<template>
  <div>
    <div class="gva-table-box">
      <div style="overflow: hidden;width: 100%;">
        <el-form
            ref="searchForm"
            :inline="true"
            :rules="rules"
            :model="traceData"
        >
          <el-form-item
              label="类名"
              prop="className"
          >
            <el-input v-model="traceData.className" @keyup.enter="handleSearch"  autocomplete="off" />
          </el-form-item>

          <el-form-item
              label="方法名"
              prop="methodName"
          >
            <el-input v-model="traceData.methodName" @keyup.enter="handleSearch" autocomplete="off" />
          </el-form-item>
          <el-form-item label-width="100px">
            <el-button size="small" type="primary" icon="refresh"   @click="handleSearch">
              查询
            </el-button>
          </el-form-item>
        </el-form>
      </div>
      <div style="overflow: hidden;width: 100%;">
        <el-card>
          <div v-if="traceHtmlData" v-html="traceHtmlData" style="background-color: black; color: white; padding: 10px;"></div>
        </el-card>
      </div>
    </div>
  </div>
</template>

<script>

export default {
  name: 'trace',
}
</script>

<script setup>
import { ref } from 'vue'
import {diagnosisProxy} from "@/plugin/kubernetes/api/diagnosis";
import { renderTraceData } from "@/plugin/kubernetes/utils/trace";

const searchInfo = ref({ 'keywords': '', 'namespace': '', 'containerName': '', 'podName': ''})
const traceData =  ref({"className": '', "methodName": '', "execNumber": 30})

const rules = ref({
  className: [{ required: true, message: '请输入类名', trigger: 'blur' }],
  methodName: [{ required: true, message: '请输入方法名', trigger: 'blur' }],
})


// 异步获取仪表板数据
const TraceStatus = ref(false)
const TracesyncExec = async() => {
  if (searchInfo.value.containerName && searchInfo.value.podName) {
    searchInfo.value['body'] = {
      "action": "async_exec",
      "command": "trace " + traceData.value.className + " "  + traceData.value.methodName,
      "sessionId" : sessionId.value
    }

    const ExecData = await diagnosisProxy(searchInfo.value)
    if (ExecData.code === 0) {
      if (ExecData.data.arthas) {
        if (ExecData.data.arthas.body?.jobStatus === "READY") {
          TraceStatus.value = true
        }else {
          TraceStatus.value = false
        }
      } else {
        TraceStatus.value = false
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
const traceHtmlData  = ref('')
const InitSearchData = async() => {
  if (!TraceStatus.value) {
    traceHtmlData.value = ''
    await initSession()
    await TracesyncExec()
  }

  if (searchInfo.value.containerName && searchInfo.value.podName && TraceStatus.value) {
    searchInfo.value['body'] = {
      "action":"pull_results",
      "sessionId" : sessionId.value,
      "consumerId": consumerId.value
    }

    const responseData = await diagnosisProxy(searchInfo.value)
    if (responseData.code === 0) {
      if (responseData.data.arthas) {
        if (responseData.data.arthas.state === "SUCCEEDED") {
          responseData.data.arthas.body.results.forEach((item, index) => {
            if (item.type === "trace") {
              traceHtmlData.value =  renderTraceData(responseData.data.arthas.body.results[0]) + traceHtmlData.value
            }
          })
        } else {
          TraceStatus.value = false
        }
      }
    }
  }
}

initSession()

setInterval(() => {
  if (TraceStatus.value) {
    InitSearchData();
  }
}, 3000);


// 查询
const handleSearch = async() => {
  TraceStatus.value = false
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
