<template>
  <div>
    <div style="margin-top: 20px; margin-bottom: 10px; text-align: right;">

      <el-button type="primary" size="small" icon="download" @click="downloadLogs">
        下载
      </el-button>
    </div>
    <div id="terminal" width="100%" style="overflow: auto;" class="console" />
  </div>
</template>
<script>
import { Terminal } from 'xterm'
import { FitAddon } from 'xterm-addon-fit'
import { ref } from 'vue'
const VITE_BASE_PATH = ref(import.meta.env.VITE_BASE_PATH)
const VITE_SERVER_PORT = ref(import.meta.env.VITE_SERVER_PORT)
export default {
  name: 'LogsConsole',
  props: {
    data: {
      type: Object,
      default() {
        return {}
      },
    },
    name: {
      type: String,
      default() {
        return ''
      }
    }
  },
  data() {
    return {
      term: null,
      terminalSocket: null,
      logData: '' // 存储终端输出的数据
    }
  },
  mounted() {
    this.initWebSocket()
  },
  beforeDestroy() {
    this.terminalSocket.close()
    this.term.destroy()
    this.logData = '' // 清理日志数据
  },
  methods: {
    initWebSocket() {
      const terminalContainer = document.getElementById('terminal')
      const height = document.body.clientHeight
      const rows = height / 18
      const cols = height / 2
      this.term = new Terminal(
        {
          cols: parseInt(cols.toString(), 10),
          rows: parseInt(rows.toString(), 10),
          rendererType: 'canvas', // 渲染类型
          convertEol: true, // 启动时，光标设置为下一行的开头
          disableStdin: false, // 是否禁用输入
          scrollback: 100, // 终端中的回滚量
          cursorBlink: true, // 光标闪烁
          cursorStyle: 'bar', // 光标样式
          fontSize: 14,
          windowsMode: true, // 根据窗口换行
          theme: {
            // 配置主题
            foreground: '#FFFFFF', // 字体
            background: '#000000', // 背景色
            cursor: 'help', // 设置光标
            lineHeight: 16,
          }
        }
      )
      const fitAddon = new FitAddon()
      this.term.loadAddon(fitAddon)
      fitAddon.fit()
      this.term.open(terminalContainer)
      // open websocket
      // const vuex_data = JSON.parse(localStorage.getItem('vuex'))
      if (VITE_BASE_PATH.value.indexOf('http') !== -1) {
        VITE_BASE_PATH.value = VITE_BASE_PATH.value.replace('http', 'ws')
      } else if (VITE_BASE_PATH.value.indexOf('https') !== -1) {
        VITE_BASE_PATH.value = VITE_BASE_PATH.value.replace('https', 'wss')
      }
      const token = localStorage.getItem('token')
      const port = VITE_SERVER_PORT?.value ? `:${VITE_SERVER_PORT.value}` : ''
      const params = new URLSearchParams({
        name: this.name,
        cluster_id: this.data.cluster_id,
        pod_name: this.data.metadata.name,
        namespace: this.data.metadata.namespace,
        'x-token': token,
        cols: this.term.cols,
        rows: this.term.rows
      }).toString()
      const url = `${VITE_BASE_PATH.value}${port}/kubernetes/pods/logs?${params}`
      this.terminalSocket = new WebSocket(url)
      this.terminalSocket.onopen = this.runRealTerminal
      this.terminalSocket.onmessage = this.onConnectionMessage
      this.terminalSocket.onclose = this.closeRealTerminal
      this.terminalSocket.onerror = this.errorRealTerminal
      this.term._initialized = true
      console.log('websocket初始化')
    },
    onConnectionMessage(event) {
      this.term.write(event.data)
      // 将数据追加到logData中，用于下载
      this.logData += event.data
    },
    runRealTerminal() {
      console.log('websocket connect onopen')
      this.term.focus()
      console.log('websocket连接成功')
    },
    errorRealTerminal() {
      console.log('error')
    },
    closeRealTerminal() {
      console.log('终端关闭连接')
    },
    downloadLogs() {
      if (!this.logData) {
        this.$message.warning('暂无日志数据可下载')
        return
      }

      // 清理ANSI转义序列，保留纯文本
      const cleanData = this.logData.replace(/\x1b\[[0-9;]*m/g, '')

      // 创建Blob对象
      const blob = new Blob([cleanData], { type: 'text/plain;charset=utf-8' })

      // 创建下载链接
      const url = window.URL.createObjectURL(blob)
      const link = document.createElement('a')
      link.href = url

      // 生成文件名
      const timestamp = new Date().toISOString().replace(/[:.]/g, '-')
      const filename = `${this.data.metadata.name}-${this.name}-${timestamp}.log`
      link.download = filename

      // 触发下载
      document.body.appendChild(link)
      link.click()
      document.body.removeChild(link)

      // 释放URL对象
      window.URL.revokeObjectURL(url)
     }
  }
}
</script>
