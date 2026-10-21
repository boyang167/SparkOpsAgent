<template>
  <div class="gva-table-box">
    <div class="gva-btn-list">
      <el-form ref="searchForm" :inline="true" :rules="rules" :model="profilerData">
        <el-form-item label="事件" prop="event">
          <el-select v-model="profilerData.event" placeholder="请选择">
            <el-option
                v-for="item in metrics"
                :key="item.value"
                :label="item.label"
                :value="item.value"
            />
          </el-select>
        </el-form-item>

        <el-form-item label-width="100px">
          <el-button
              v-if="!profilerStatus"
              size="small"
              type="primary"
              icon="CaretRight"
              @click="handleProfilerStart"
          >
            启动
          </el-button>
          <el-button
              v-if="profilerStatus"
              size="small"
              type="danger"
              icon="Delete"
              @click="handleProfilerStop"
          >
            关闭
          </el-button>
          <el-button
              v-if="resultfileName"
              size="small"
              type="danger"
              icon="Download"
              @click="handleProfilerDownload"
          >
            下载
          </el-button>
        </el-form-item>
      </el-form>
    </div>
    <el-card v-if="profilerHtmlData" style="background-color: black; color: white; padding: 10px;">
      <div v-html="profilerHtmlData"></div>
    </el-card>
  </div>
</template>

<script setup>
import { ref, onMounted, onUnmounted } from 'vue'
import { diagnosisProxy } from "@/plugin/kubernetes/api/diagnosis"
import { generateFileName } from "@/plugin/kubernetes/utils/profiler"

const VITE_BASE_PATH = ref(import.meta.env.VITE_BASE_PATH)
const VITE_SERVER_PORT = ref(import.meta.env.VITE_SERVER_PORT)
const searchInfo = ref({ keywords: '', namespace: '', containerName: '', podName: '' })
const profilerData = ref({ event: 'cpu' })
const profilerHtmlData = ref('')
const profilerStatus = ref(false)
const profilerStart = ref(false)
const resultfileName = ref('')
const rules = ref({
  event: [{ required: true, message: 'cpu, alloc, lock, cache-misses 等，（默认是 cpu)', trigger: 'blur' }],
})
const metrics = ref([
  { label: 'CPU', value: 'cpu' },
  { label: 'Alloc', value: 'alloc' },
  { label: 'Lock', value: 'lock' },
  { label: 'Wall', value: 'wall' },
  { label: 'Itimer', value: 'itimer' },
  { label: 'Ctimer', value: 'ctimer' },
])

const InitExecData = async (command) => {
  if (!searchInfo.value.containerName || !searchInfo.value.podName) return

  searchInfo.value.body = { action: 'exec', command }
  const responseData = await diagnosisProxy(searchInfo.value)

  if (responseData.code === 0 && responseData.data.arthas) {
    responseData.data.arthas.body.results.forEach(item => {
      if (item.executeResult) {
        profilerHtmlData.value = item.executeResult + "<br>" + profilerHtmlData.value
        if (item.executeResult.includes("Profiling is running for")) {
          profilerStatus.value = true
        }
      }
    })
  }
}

const pollProfilerStatus = () => {
  if (profilerStart.value) InitExecData("profiler status")
}

const handleProfilerStart = async () => {
  if (!searchInfo.value.containerName || !searchInfo.value.podName) return

  profilerStatus.value = true
  profilerHtmlData.value = ''
  resultfileName.value = ''
  profilerStart.value = true
  await InitExecData(`profiler start --event ${profilerData.value.event}`)
}

const handleProfilerStop = async () => {
  if (!searchInfo.value.containerName || !searchInfo.value.podName) return

  profilerStatus.value = false
  profilerStart.value = false
  resultfileName.value = generateFileName()
  await InitExecData(`profiler stop --file /tmp/${resultfileName.value}`)
}

const GetUrl = () => {
  const queryString = Object.keys(searchInfo.value)
      .map(key => `${key}=${searchInfo.value[key]}`)
      .join('&')
  return `?${queryString}&path=/tmp/${resultfileName.value}`
}

const handleProfilerDownload = () => {
  if (searchInfo.value.containerName && searchInfo.value.podName) {
    const url = `${VITE_BASE_PATH.value}:${VITE_SERVER_PORT.value}/kubernetes/pods/downloadFile${GetUrl()}&x-token=${localStorage.getItem('token')}`
    window.open(url, '_blank')
  }
}

const updateSearchInfo = () => {
  searchInfo.value.cluster_id = parseInt(localStorage.getItem('cluster_id'))
  searchInfo.value.namespace = localStorage.getItem('namespace')
  searchInfo.value.podName = localStorage.getItem('podName')
  searchInfo.value.containerName = localStorage.getItem('containerName')
}

onMounted(() => {
  updateSearchInfo()
  const intervalId = setInterval(pollProfilerStatus, 3000)
  onUnmounted(() => clearInterval(intervalId))
})

window.addEventListener('setItemCluster', updateSearchInfo)
window.addEventListener('setItemNamespace', updateSearchInfo)
window.addEventListener('setItempodName', updateSearchInfo)
window.addEventListener('setItemcontainerName', updateSearchInfo)
</script>

<style scoped>
.gva-table-box {
  display: flex;
  flex-direction: column;
  gap: 1rem;
}

.gva-btn-list {
  display: flex;
  gap: 1rem;
}

.button-container {
  display: flex;
  justify-content: center;
  align-items: center;
}
</style>
