<template>
  <div>
    <el-table ref="multipleTable" :data="tableData" style="width: 100%" tooltip-effect="dark" row-key="ID">
      <el-table-column type="selection" width="55" />
      <el-table-column align="left" label="名称" prop="metadata.name">
        <template #default="scope">
          <span class="operate-span-primary" @click="handleDetail(scope.row)">{{ scope.row.metadata.name }}</span>
          <el-tooltip placement="top">
            <template #content>
              <div v-for="(v,k, i) in scope.row.metadata.labels" slot="content" :key="i">
                <span> {{ k }}: {{ v }}</span>
              </div>
            </template>
            <el-button size="small" type="primary" link icon="PriceTag" />
          </el-tooltip>
        </template>
      </el-table-column>
      <el-table-column label="Internal IP" prop="metadata.name" max-width="30px">
        <template #default="scope">
          <div v-for="(address,index) in scope.row.status.addresses" :key="index">
            <span v-if="address.type === 'InternalIP'">{{ address.address }}</span>
          </div>
        </template>
      </el-table-column>
      <el-table-column label="状态" prop="status" type="scope">
        <template #default="scope">
          <el-tag v-if="handleNodeStatus(scope.row) ==='Ready'" type="success">Ready</el-tag>
          <el-tag v-else type="danger">{{ handleNodeStatus(scope.row) }}</el-tag>
        </template>
      </el-table-column>
      <el-table-column label="角色" prop="metadata.labels" type="scope">
        <template #default="scope">
          <span>{{ handleNodeRole(scope.row.metadata.labels) }}</span>
        </template>
      </el-table-column>
      <el-table-column prop="cpuRequestsPersent" type="scope">
        <template #header>
          <div style="line-height: 1">
            <div>CPU</div>
            <div style="font-size: 12px; color: #666">申请/限制</div>
          </div>
        </template>
        <template #default="scope">
          <div class="percent-box">
            <div class="percent-top">{{ toPercent(scope.row.cpuRequestsPersent) }}</div>
            <div class="dashed-sep"></div>
            <div class="percent-bottom" :class="{ overcommit: (Number(scope.row.cpuLimitsPersent) || 0) > 100 }">{{ toPercent(scope.row.cpuLimitsPersent) }}</div>
          </div>
        </template>
      </el-table-column>
      <el-table-column prop="memoryRequestsPersent" type="scope">
        <template #header>
          <div style="line-height: 1">
            <div>内存</div>
            <div style="font-size: 12px; color: #666">申请/限制</div>
          </div>
        </template>
        <template #default="scope">
          <div class="percent-box">
            <div class="percent-top">{{ toPercent(scope.row.memoryRequestsPersent) }}</div>
            <div class="dashed-sep"></div>
            <div class="percent-bottom" :class="{ overcommit: (Number(scope.row.memoryLimitsPersent) || 0) > 100 }">{{ toPercent(scope.row.memoryLimitsPersent) }}</div>
          </div>
        </template>
      </el-table-column>
      <el-table-column label="版本" prop="version" type="scope" align="center" width="250">
        <template #default="scope">
          <div style="text-align: center">
            <span>{{ scope.row.status.nodeInfo.kubeletVersion }}</span><br>
            <span>{{ scope.row.status.nodeInfo.containerRuntimeVersion }}</span><br>
            <span>{{ scope.row.status.nodeInfo.osImage }}</span>
          </div>
        </template>
      </el-table-column>
      <el-table-column align="left" label="存活时间" prop="metadata.creationTimestamp" width="180">
        <template #default="scope">
          <span>{{ AgeFormat(scope.row.metadata.creationTimestamp) }}</span>
        </template>
      </el-table-column>
      <el-table-column align="left" label="操作">
        <template #default="scope">
          <el-form :inline="true" class="demo-form-inline">
            <el-form-item>
              <el-button size="small" type="primary" link icon="edit" @click="handleUpdate(scope.row)">编辑</el-button>
              <el-dropdown>
                <el-button type="primary" link icon="MoreFilled" size="small">更多</el-button>
                <template #dropdown>
                  <el-dropdown-menu>
                    <el-dropdown-item icon="InfoFilled" @click.native="handleDetail(scope.row)">详情</el-dropdown-item>
                    <el-dropdown-item icon="Expand" @click.native="handleShell(scope.row)">Shell</el-dropdown-item>
                    <el-dropdown-item icon="Monitor" @click.native="handleMonitor(scope.row)">监控</el-dropdown-item>
                    <el-dropdown-item icon="edit" @click.native="handleYAML(scope.row)">YAML</el-dropdown-item>
                    <el-dropdown-item icon="Crop" @click.native="handleDrain(scope.row)">节点排空</el-dropdown-item>
                    <el-dropdown-item icon="Setting" @click.native="handleSchedule(scope.row)">调度设置</el-dropdown-item>
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
    drain: () => {
      return true
    },
    schedule: () => {
      return true
    },
    detail: () => {
      return true
    },
    yaml: () => {
      return true
    },
    monitor: () => {
      return true
    },
    shell: () => {
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
import { AgeFormat } from '@/plugin/kubernetes/utils/age'
import { giMemoryFormat, CpuFormat } from '@/plugin/kubernetes/utils/unitConvert'
const emit = defineEmits(['update', 'delete', 'search', 'detail', 'schedule', 'drain', 'yaml', 'monitor', 'shell'])
defineProps({
  tableData: {
    default: function() {
      return []
    },
    type: Array
  },
})

// 更新
const handleUpdate = (value) => {
  emit('update', value)
}

// 删除
const handleDelete = (value) => {
  emit('delete', value)
}

// 节点状态
const handleNodeStatus = (value) => {
  let status = ''
  for (const condition of value.status.conditions) {
    if (condition.type === 'Ready') {
      if (condition.status === 'True') {
        status = 'Ready'
      }
      break
    }
  }

  if (value.spec.unschedulable) {
    status += ', SchedulingDisabled'
  }
  return status
}

// 节点角色
const handleNodeRole = (labels) => {
  if (!labels || typeof labels !== 'object') {
    return ''
  }
  const roleLabels = Object.keys(labels)
    .filter(key => key.includes('node-role.kubernetes.io'))
    .map(key => key.match(/([^/]+$)/)[0]) // all after last slash

  if (labels['kubernetes.io/role'] !== undefined) {
    roleLabels.push(labels['kubernetes.io/role'])
  } else {
    roleLabels.push('Worker')
  }
  return roleLabels.join(', ')
}

// 排空
const handleDrain = (value) => {
  emit('drain', value)
}

// 调度设置
const handleSchedule = (value) => {
  emit('schedule', value)
}

// 详情
const handleDetail = (value) => {
  emit('detail', value)
}

// YAML
const handleYAML = (value) => {
  emit('yaml', value)
}

// Monitor
const handleMonitor = (value) => {
  emit('monitor', value)
}

// Shell
const handleShell = (value) => {
  emit('shell', value)
}

// 百分比格式化（两位小数）
const toPercent = (val) => {
  const n = Number(val)
  if (isNaN(n)) {
    return '0%'
  }
  return `${n.toFixed(2)}%`
}
</script>
<style scoped>
.el-dropdown-link {
  cursor: pointer;
  color: var(--el-color-primary);
  display: flex;
  align-items: center;
}
.tooltip-base-box {
  width: 600px;
}
.tooltip-base-box .row {
  display: flex;
  align-items: center;
  justify-content: space-between;
}
.tooltip-base-box .center {
  justify-content: center;
}
.tooltip-base-box .box-item {
  width: 110px;
  margin-top: 10px;
}
.percent-box {
  display: flex;
  flex-direction: column;
  align-items: center;
}
.percent-top {
  font-size: 14px;
}
.dashed-sep {
  width: 64px;
  border-bottom: 2px dashed #bbb;
  margin: 6px 0;
}
.percent-bottom {
  font-size: 14px;
}
.overcommit {
  color: var(--el-color-danger);
}
.operate-span-primary {
  color: var(--el-color-primary);
  cursor: pointer;
  font-weight:bold;
}
</style>
