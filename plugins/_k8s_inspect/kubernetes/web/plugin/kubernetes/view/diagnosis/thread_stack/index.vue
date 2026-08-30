<template>
  <div>
    <div class="gva-table-box">
      <div style="overflow: hidden;width: 100%;">
        <el-form
          ref="searchForm"
          :inline="true"
          :rules="rules"
          :model="threadData"
        >
          <el-form-item
            label="线程ID"
            prop="thread_id"
          >
            <el-input v-model="threadData.thread_id" @keyup.enter="handleSearch" style="width: 200px" autocomplete="off" />
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
          <div v-if="threadStackData" v-html="threadStackData" style="background-color: black; color: white; padding: 10px;"></div>
        </el-card>
      </div>
    </div>
  </div>
</template>

<script>

export default {
  name: 'ThreadStack',
}
</script>

<script setup>
import { ref } from 'vue'

import {diagnosisProxy} from "@/plugin/kubernetes/api/diagnosis";


const searchInfo = ref({ 'keywords': '', 'namespace': '', 'containerName': '', 'podName': ''})
const threadData =  ref({"thread_id": ''})
const threadStackData =  ref('')

const rules = ref({
  thread_id: [{ required: true, message: '请输入进程ID', trigger: 'blur' }],
})

// 初始化数据
const InitSearchData = async() => {
  threadStackData.value = ''
  if (searchInfo.value.containerName && searchInfo.value.podName) {
    searchInfo.value['body'] = {
      "action":"exec",
      "command": "thread " + threadData.value.thread_id
    }

    const responseData = await diagnosisProxy(searchInfo.value)
    if (responseData.code === 0) {
      if (responseData.data.arthas) {
        if (responseData.data.arthas.state === "SUCCEEDED") {
          responseData.data.arthas.body.results.forEach((item, index) => {
            if (item.threadInfo) {
              threadStackData.value = '"' + item.threadInfo.threadName + '"' + " Id=" + item.threadInfo.threadId + " " + item.threadInfo.threadState  + " "  + item.threadInfo.lockName+'<br>';
              threadStackData.value += item.threadInfo.stackTrace.map(item => {
                return `&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;at ${item.className}.${item.methodName}(${item.fileName}:${item.lineNumber})`;
              }).join('<br>');
              threadStackData.value += '<br><br>';
            }
          })
        }
      }
    }
  }
}


// 查询
const handleSearch = async() => {
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
})
</script>
<style>
.button-container {
  display: flex;              /* 使用 Flexbox */
  justify-content: center;    /* 水平居中 */
  align-items: center;        /* 垂直居中（如果有需要的话） */
}
</style>
