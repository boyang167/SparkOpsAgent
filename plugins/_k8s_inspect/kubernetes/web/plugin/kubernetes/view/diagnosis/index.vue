<template>
  <div class="container">
    <ClusterNamespacePods />
    <el-tabs style="width:100%;">
      <el-tab-pane label="控制面板">
        <!--子组件-->
        <threadsDashboardBlock v-if="threadsData" :table-data="threadsData" />
        <memoryInfoDashboardBlock v-if="memoryInfoData" :table-data="memoryInfoData" />
        <gcInfosDashboardBlock v-if="gcInfosData" :table-data="gcInfosData" />
        <runtimeInfoDashboardBlock  v-if="runtimeInfoData" :data="runtimeInfoData" />
      </el-tab-pane>
      <el-tab-pane label="线程清单">
        <threadsBlock />
        <!--子组件-->
      </el-tab-pane>
      <el-tab-pane label="JVM信息">
        <jvmBlock />
        <!--子组件-->
      </el-tab-pane>
      <el-tab-pane label="性能计数器">
        <perfcounterBlock />
        <!--子组件-->
      </el-tab-pane>
      <el-tab-pane label="系统信息">
        <syspropBlock />
        <!--子组件-->
      </el-tab-pane>
      <el-tab-pane label="线程堆栈">
        <sthreadStackBlock />
        <!--子组件-->
      </el-tab-pane>
      <el-tab-pane label="火焰图">
        <profilerBlock />
        <!--子组件-->
      </el-tab-pane>
      <el-tab-pane label="方法追踪">
        <traceBlock />
        <!--子组件-->
      </el-tab-pane>
      <el-tab-pane label="方法监测">
        <watchBlock />
        <!--子组件-->
      </el-tab-pane>
      <el-tab-pane label="方法监控">
        <monitorBlock />
        <!--子组件-->
      </el-tab-pane>
      <el-tab-pane label="源码查看">
        <sourcecodeBlock />
        <!--子组件-->
      </el-tab-pane>
      <el-tab-pane label="静态属性">
        <getstaticBlock />
        <!--子组件-->
      </el-tab-pane>
    </el-tabs>
  </div>
</template>
<script>
import threadsDashboardBlock from './dashboard/threads/index.vue'
import memoryInfoDashboardBlock from './dashboard/memoryInfo/index.vue'
import gcInfosDashboardBlock from './dashboard/gcInfos/index.vue'
import runtimeInfoDashboardBlock from './dashboard/runtimeInfo/table.vue'
import threadsBlock from './threads/index.vue'
import jvmBlock from './jvm/index.vue'
import perfcounterBlock from  './perfcounter/index.vue'
import syspropBlock from './sysprop/index.vue'
import sthreadStackBlock from './thread_stack/index.vue'
import profilerBlock from './profiler/index.vue'
import traceBlock from './trace/index.vue'
import watchBlock from './watch/index.vue'
import monitorBlock from './monitor/index.vue'
import sourcecodeBlock from './sourcecode/index.vue'
import getstaticBlock from './getstatic/index.vue'
import ClusterNamespacePods from '@/plugin/kubernetes/components/diagnosis/select/index.vue'
export default {
  // eslint-disable-next-line vue/multi-word-component-names
  name: '仪表盘',
  components: {
    threadsBlock,
    jvmBlock,
    perfcounterBlock,
    syspropBlock,
    sthreadStackBlock,
    profilerBlock,
    traceBlock,
    watchBlock,
    monitorBlock,
    sourcecodeBlock,
    getstaticBlock,
    threadsDashboardBlock,
    memoryInfoDashboardBlock,
    runtimeInfoDashboardBlock,
    gcInfosDashboardBlock,
    ClusterNamespacePods
  },
}
</script>
<script setup>
import { ref } from 'vue'
import { diagnosisProxy } from "@/plugin/kubernetes/api/diagnosis";
import {ElMessage} from "element-plus";

const searchInfo = ref({})
const consumerId = ref('')
const sessionId = ref('')
const dashboardStatus = ref(false)
let timerId = null;

// pull_results 获取异步数据
const gcInfosData = ref([])
const memoryInfoData = ref([])
const runtimeInfoData = ref({})
const threadsData = ref([])

const DashboardGet = async() => {
  if (searchInfo.value.containerName && searchInfo.value.podName && dashboardStatus) {
    searchInfo.value['body'] = {
      "action":"pull_results",
      "sessionId" : sessionId.value,
      "consumerId": consumerId.value
    }

    const responseData = await diagnosisProxy(searchInfo.value)
    if (responseData.code === 0) {
      if (responseData.data.arthas) {
        if (responseData.data.arthas.state === "SUCCEEDED") {
          gcInfosData.value = responseData.data?.arthas?.body?.results[0]?.gcInfos || [];
          responseData.data?.arthas?.body?.results.flatMap(item => {
            memoryInfoData.value= [].concat(item.memoryInfo?.buffer_pool || [], item.memoryInfo?.heap || [], item.memoryInfo?.nonheap)
          })
          runtimeInfoData.value = responseData.data?.arthas?.body?.results[0]?.runtimeInfo || {};
          threadsData.value = responseData.data?.arthas?.body?.results[0]?.threads || [];
        } else {
          await initSession()
        }
      }
    }
  }
}

// 每隔3秒调用一次 DashboardGet 函数
setInterval(() => DashboardGet(), 10000);

// 异步获取仪表板数据
const DashboardasyncExec = async() => {
  if (searchInfo.value.containerName && searchInfo.value.podName) {
    searchInfo.value['body'] = {
      "action": "async_exec",
      "command":"dashboard",
      "sessionId" : sessionId.value
    }

    const ExecData = await diagnosisProxy(searchInfo.value)
    if (ExecData.code === 0) {
      if (ExecData.data.arthas) {
        if (ExecData.data.arthas.body.jobStatus !== "READY") {
          ElMessage({
            type: 'error',
            message: "异步执行dashboard 失败请联系管理员",
          })
        } else {
          dashboardStatus.value = true
        }
      }
    }
  }
}

// 初始化session
const initSession = async() => {
  if (searchInfo.value.containerName && searchInfo.value.podName) {
    searchInfo.value['body'] = {
      "action": "init_session"
    }
    const dashboardData = await diagnosisProxy(searchInfo.value)
    if (dashboardData.code === 0) {
      if (dashboardData.data.arthas) {
        consumerId.value = dashboardData.data.arthas.consumerId
        sessionId.value = dashboardData.data.arthas.sessionId
        await DashboardasyncExec()
      }
    }
  }
}

// 检查条件并执行任务
const checkAndExecute = () => {
  if (searchInfo.value.containerName && searchInfo.value.podName) {
    clearTimeout(timerId);  // 清除之前的定时器
    initSession();  // 满足条件时立即执行一次
  } else {
    // 不满足条件时，每20分钟执行一次
    timerId = setTimeout(() => {
      initSession();
      checkAndExecute();  // 重新检查条件
    }, 20 * 60 * 1000);  // 20分钟的延迟 (1200000毫秒)
  }
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
  checkAndExecute();  // 检查条件并执行任务
})

window.addEventListener('setItemcontainerName', (e) => {
  searchInfo.value.containerName = localStorage.getItem('containerName')
  checkAndExecute();  // 检查条件并执行任务
})

// 初始化时检查条件
checkAndExecute();

</script>
