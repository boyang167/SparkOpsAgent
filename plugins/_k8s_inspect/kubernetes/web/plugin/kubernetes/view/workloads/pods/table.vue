<template>
  <div>
    <el-table ref="multipleTable" :key="tableData" :data="tableData" style="width: 100%" tooltip-effect="dark" row-key="ID">
      <el-table-column type="selection" width="55" />
      <el-table-column align="left" label="名称" prop="metadata.name" min-width="360">
        <template #default="scope">
          <div class="name-cell-row">
            <img style="width:14px;margin-right:2px" src="//g.alicdn.com/aliyun/cos/1.38.27/images/icon_docker.png">
            <span class="operate-span-primary name-text" :title="scope.row.metadata.name" @click="handleDetail(scope.row)">{{ scope.row.metadata.name }}</span>
            <el-tooltip placement="top">
              <template #content>
                <div v-for="(v,k, i) in scope.row.metadata.labels" slot="content" :key="i">
                  <span> {{ k }}: {{ v }}</span>
                </div>
              </template>
              <el-button size="small" type="primary" link icon="PriceTag" />
            </el-tooltip>
          </div>
        </template>
      </el-table-column>
      <el-table-column
        label="CPU 申请/限制"
        prop="cpuReqLim"
        type="scope"
        
      >
        <template #header>
          <div class="col-header">
            <div class="col-header-title">CPU</div>
            <div class="col-header-sub">申请/限制</div>
          </div>
        </template>
        <template #default="scope">
          <div class="percent-stack">
            <div class="percent-line">{{ CpuFormat(podCpuReq(scope.row)) }} Cores</div>
            <div class="percent-line dashed"></div>
            <div :class="['percent-line', cpuOver(scope.row) ? 'percent-over' : '']">{{ CpuFormat(podCpuLim(scope.row)) }} Cores</div>
          </div>
        </template>
      </el-table-column>
      <el-table-column
        label="内存 申请/限制"
        prop="memReqLim"
        type="scope"
        
      >
        <template #header>
          <div class="col-header">
            <div class="col-header-title">内存</div>
            <div class="col-header-sub">申请/限制</div>
          </div>
        </template>
        <template #default="scope">
          <div class="percent-stack">
            <div class="percent-line">{{ MiIntFormat(podMemReq(scope.row)) }} MiB</div>
            <div class="percent-line dashed"></div>
            <div :class="['percent-line', memOver(scope.row) ? 'percent-over' : '']">{{ MiIntFormat(podMemLim(scope.row)) }} MiB</div>
          </div>
        </template>
      </el-table-column>
      <el-table-column label="容器" prop="container" type="scope">
        <template #default="scope">
          <div v-for="(p, index) in scope.row.status.containerStatuses" :key="index" class="gridChart">
            <div v-if="p.ready">
              <el-tooltip placement="top">
                <template #content>
                  <div slot="content">
                    {{ p.name }}（running, ready）
                  </div>
                </template>
                <el-tag class="ml-2" type="success" effect="dark" />
              </el-tooltip>
            </div>
            <div v-else-if="!p.ready && p.state.waiting">
              <el-tooltip placement="top">
                <template #content>
                  <div slot="content">
                    {{ p.name }} 报错信息: {{ p.state.waiting.reason }}
                  </div>
                </template>
                <el-tag class="ml-2" type="danger" effect="dark" />
              </el-tooltip>
            </div>
            <div v-else-if="!p.ready && p.state.terminated">
              <el-tooltip placement="top">
                <template #content>
                  <div slot="content">
                    {{ p.name }} {{ p.state.terminated.reason }}
                  </div>
                </template>
                <el-tag class="ml-2" type="info" effect="dark" />
              </el-tooltip>
            </div>
            <div v-else>
              <el-tooltip placement="top">
                <template #content>
                  <div slot="content">
                    {{ p.name }}
                  </div>
                </template>
                <el-tag class="ml-2" type="info" effect="dark" />
              </el-tooltip>
            </div>
          </div>
        </template>
      </el-table-column>
      <el-table-column
        prop="status"
        label="CPU/内存/TCP"
        type="scope"
        
      >
        <template #default="scope">
          <div v-if="scope.row">
            <BarMonitorIndex :data="scope.row" :cluster_id="cluster_id" />
          </div>
        </template>
      </el-table-column>
      <el-table-column
        label="状态"
        prop="status"
        type="scope"
        
      >
        <template #default="scope">
          <el-tag v-if="scope.row.status.phase ==='Running'" type="success">Running</el-tag>
          <el-tag v-else-if="scope.row.status.phase ==='Succeeded'" type="info">Completed</el-tag>
          <el-tag v-else type="danger">{{ scope.row.status.phase }}</el-tag>
        </template>
      </el-table-column>
      <el-table-column
        label="重启次数"
        prop="restarts"
        type="scope"
      >
        <template #default="scope">
          <span>{{ RestartCounts(scope.row.status.containerStatuses) }}</span>
        </template>
      </el-table-column>
      <el-table-column
        label="命名空间"
        prop="namespace"
        type="scope"
        
      >
        <template #default="scope">
          <span>{{ scope.row.metadata.namespace }}</span>
        </template>
      </el-table-column>
      <el-table-column
        label="Pod IP"
        prop="Pod_IP"
        type="scope"
        
      >
        <template #default="scope">
          <span>{{ scope.row.status.podIP }}</span>
        </template>
      </el-table-column>
      <el-table-column
        label="调度节点"
        prop="hostIP"
        type="scope"
        show-overflow-tooltip
      >
        <template #default="scope">
          <span class="operate-span">{{ scope.row.status.hostIP }}</span>
        </template>
      </el-table-column>
      <el-table-column align="left" label="存活时间" prop="metadata.creationTimestamp">
        <template #default="scope">
          <span>{{ AgeFormat(scope.row.metadata.creationTimestamp) }}</span>
        </template>
      </el-table-column>
      <el-table-column align="left" label="操作" fixed="right" width="160">
        <template #default="scope">
          <el-form :inline="true" class="demo-form-inline">
            <el-form-item>
              <el-button size="small" type="primary" link icon="edit" @click="handleUpdate(scope.row)">编辑</el-button>
              <el-dropdown>
                <el-button type="primary" link icon="MoreFilled" size="small">更多</el-button>
                <template #dropdown>
                  <el-dropdown-menu>
                    <el-dropdown-item icon="InfoFilled" @click.native="handleDetail(scope.row)">详情</el-dropdown-item>
                    <div v-for="(item,index) in scope.row.spec.containers" :key="index">
                      <el-dropdown-item icon="Files" @click.native="handleFiles(scope.row, item.name)">
                        {{ item.name }} 文件浏览
                      </el-dropdown-item>
                    </div>
                    <el-dropdown-item icon="Monitor" @click.native="handleMonitor(scope.row)">监控</el-dropdown-item>
                    <div v-for="(item,index) in scope.row.spec.containers" :key="index">
                      <el-dropdown-item icon="Expand" @click.native="handleTerminal(scope.row, item.name)">
                        {{ item.name }} 终端
                      </el-dropdown-item>
                    </div>
                    <div v-for="(item,index) in scope.row.spec.containers" :key="index">
                      <el-dropdown-item icon="Expand" @click.native="handleTerminalLogs(scope.row, item.name)">
                        {{ item.name }} 终端日志
                      </el-dropdown-item>
                    </div>
                    <el-dropdown-item icon="delete" @click.native="handleDelete(scope.row)">删除</el-dropdown-item>
                  </el-dropdown-menu>
                </template>
              </el-dropdown>
            </el-form-item>
          </el-form>
        </template>
      </el-table-column>
    </el-table>
  </div>
</template>

<script>
export default {
  name: 'TableBlock',
  emits: {
    update: () => {
      return true
    },
    search: () => {
      return true
    },
    detail: () => {
      return true
    },
    terminal: () => {
      return true
    },
    terminallog: () => {
      return true
    },
    delete: (value) => {
      if (value.length === 1) {
        return true
      } else {
        return false
      }
    },
  }
}
</script>
<script setup>
import BarMonitorIndex from '@/plugin/kubernetes/components/monitor/pods/index.vue'
import { AgeFormat } from '@/plugin/kubernetes/utils/age'
import { RestartCounts } from '@/plugin/kubernetes/utils/restart'
import { CpuFormat, cpuUnitConvert, memoryUnitConvert } from '@/plugin/kubernetes/utils/unitConvert'
const emit = defineEmits(['update', 'delete', 'search', 'detail', 'terminal', 'terminallog', 'files'])
defineProps({
  tableData: {
    default: function() {
      return []
    },
    type: Array
  },
  cluster_id: {
    default: function() {
      return 0
    },
    type: Number
  }
})

// 更新
const handleUpdate = (value) => {
  emit('update', value)
}

// 删除
const handleDelete = (value) => {
  emit('delete', value)
}

// 详情
const handleDetail = (value) => {
  emit('detail', value)
}

// 终端
const handleTerminal = (value, name) => {
  emit('terminal', value, name)
}

// 终端日志
const handleTerminalLogs = (value, name) => {
  emit('terminallog', value, name)
}

// Monitor
const handleMonitor = (value) => {
  emit('monitor', value)
}

// 文件浏览
const handleFiles = (value, name) => {
  emit('files', value, name)
}

// 整个 Pod 资源聚合（containers + initContainers + ephemeralContainers）
const sumPodResources = (pod) => {
  let cpuReq = 0
  let cpuLim = 0
  let memReq = 0
  let memLim = 0
  const containers = [
    ...(pod.spec?.containers || []),
    ...(pod.spec?.initContainers || []),
    ...(pod.spec?.ephemeralContainers || []),
  ]
  for (const c of containers) {
    const r = c.resources || {}
    if (r.requests?.cpu) cpuReq += cpuUnitConvert(r.requests.cpu)
    if (r.limits?.cpu) cpuLim += cpuUnitConvert(r.limits.cpu)
    if (r.requests?.memory) memReq += memoryUnitConvert(r.requests.memory)
    if (r.limits?.memory) memLim += memoryUnitConvert(r.limits.memory)
  }
  return { cpuReq, cpuLim, memReq, memLim }
}

const podCpuReq = (pod) => `${sumPodResources(pod).cpuReq}m`
const podCpuLim = (pod) => `${sumPodResources(pod).cpuLim}m`
const podMemReq = (pod) => `${sumPodResources(pod).memReq}Mi`
const podMemLim = (pod) => `${sumPodResources(pod).memLim}Mi`

// 将值统一显示为 MiB（两位小数）
const MiBFormat = (value) => {
  if (value == null) return ''
  try {
    const mi = memoryUnitConvert(String(value))
    return Number(mi).toFixed(2)
  } catch (e) {
    return '0.00'
  }
}

// 内存整数显示（MiB，四舍五入为整数）
const MiIntFormat = (value) => {
  if (value == null) return ''
  try {
    const mi = memoryUnitConvert(String(value))
    return String(Math.round(Number(mi)))
  } catch (e) {
    return '0'
  }
}

// 底部“限制”是否需要红色高亮（当申请 > 限制时）
const cpuOver = (pod) => {
  const { cpuReq, cpuLim } = sumPodResources(pod)
  return cpuLim > 0 && cpuReq > cpuLim
}
const memOver = (pod) => {
  const { memReq, memLim } = sumPodResources(pod)
  return memLim > 0 && memReq > memLim
}
</script>
<style scoped>
  .col-header {
    display: flex;
    flex-direction: column;
    line-height: 1.1;
  }
  .col-header-title {
    font-size: 14px;
    font-weight: 700;
    color: var(--el-text-color-primary);
  }
  .col-header-sub {
    font-size: 12px;
    color: var(--el-text-color-secondary);
  }
  :deep(.el-table .cell) {
    white-space: normal;
    word-break: break-all;
    overflow: visible;
  }
  .operate-span-primary {
    color: var(--el-color-primary);
    cursor: pointer;
    font-weight:bold;
  }
  .name-cell-row {
    display: flex;
    align-items: center;
    gap: 4px;
  }
  .name-text {
    flex: 1;
    white-space: normal;
    word-break: break-all;
    overflow: visible;
  }
  .percent-stack {
    display: flex;
    flex-direction: column;
    line-height: 1.15;
  }
  .percent-line {
    font-size: 11px;
    color: var(--el-text-color-primary);
  }
  .percent-line.dashed {
    border-bottom: 1px dashed var(--el-border-color);
    margin: 2px 0;
    height: 0;
  }
  .percent-over {
    color: var(--el-color-danger);
    font-weight: 500;
  }
</style>
