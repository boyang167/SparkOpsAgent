<template>
  <div class="chat-container">
    <!-- 聊天记录显示区域 -->
    <div class="chat-messages" ref="messagesContainer">
      <div 
        v-for="(message, index) in chatHistory" 
        :key="index" 
        class="message-item"
        :class="message.type"
      >
        <div class="message-avatar">
          <el-icon v-if="message.type === 'user'" size="20">
            <User />
          </el-icon>
          <el-icon v-else size="20">
             <Service />
          </el-icon>
        </div>
        <div class="message-content">
          <div class="message-header">
            <el-button
              v-if="message.type === 'assistant' && message.content"
              type="text"
              size="small"
              @click="copyToClipboard(message.content)"
              class="copy-button"
            >
              <el-icon><DocumentCopy /></el-icon>
            </el-button>
          </div>
          <div class="message-text">{{ message.content }}</div>
          <div class="message-time">{{ formatTime(message.timestamp) }}</div>
        </div>
      </div>
      
      <!-- 正在输入指示器 -->
      <div v-if="isLoading" class="message-item assistant">
        <div class="message-avatar">
           <el-icon size="20">
             <Service />
          </el-icon>
        </div>
        <div class="message-content">
          <div class="typing-indicator">
            <span></span>
            <span></span>
            <span></span>
          </div>
        </div>
      </div>
    </div>
    
    <!-- 快捷操作按钮区域 -->
    <div class="quick-actions-area">
      <div class="quick-actions-title">常用操作</div>
      <div class="quick-actions-buttons">
        <el-button
          v-for="action in quickActions"
          :key="action.id"
          size="small"
          type="primary"
          plain
          @click="selectQuickAction(action)"
          class="quick-action-btn"
        >
          <el-icon><component :is="action.icon" /></el-icon>
          {{ action.label }}
        </el-button>
      </div>
    </div>

    <!-- 输入区域 -->
    <div class="chat-input-area">
      <div class="input-container">
        <el-input
          v-model="userInput"
          type="textarea"
          :rows="2"
          placeholder="请输入您的问题..."
          :disabled="isLoading"
          @keydown.enter.prevent="handleKeyDown"
          class="chat-input"
        />
        <el-button
          type="primary"
          :loading="isLoading"
          :disabled="!userInput.trim()"
          @click="sendMessage"
          class="send-button"
        >
          <el-icon><Promotion /></el-icon>
        </el-button>
      </div>
    </div>
  </div>
</template>

<script>
export default {
  name: "ChatComponent",
}
</script>

<script setup>
import { ref, reactive, nextTick, onMounted } from 'vue'
import { ChatStream } from '@/plugin/kubernetes/api/chat'
import { User, Service, Promotion, DocumentCopy, List, View, Document, FolderOpened, Monitor } from '@element-plus/icons-vue'
import { ElMessage } from 'element-plus'

// 响应式数据
const userInput = ref('')
const isLoading = ref(false)
const chatHistory = ref([])
const messagesContainer = ref(null)
const clusterId = ref(null)

// 快捷操作定义
const quickActions = ref([
  {
    id: 'cluster-status',
    label: '查看集群状态',
    icon: 'Monitor',
    question: '查看集群状态'
  },
  {
    id: 'pods',
    label: '查看所有Pods',
    icon: 'List',
    question: '查看所有Pods'
  },
  {
    id: 'services',
    label: '查看所有Services',
    icon: 'View',
    question: '查看所有Services'
  },
  {
    id: 'deployments',
    label: '查看所有Deployments',
    icon: 'Document',
    question: '查看所有Deployments'
  },
  {
    id: 'nodes',
    label: '查看所有Nodes',
    icon: 'FolderOpened',
    question: '查看所有Nodes'
  },
  {
    id: 'namespaces',
    label: '查看所有Namespaces',
    icon: 'Monitor',
    question: '查看所有Namespaces'
  },
  {
    id: 'configmaps',
    label: '查看所有ConfigMaps',
    icon: 'Document',
    question: '查看所有ConfigMaps'
  }
])

// 选择快捷操作
const selectQuickAction = (action) => {
  userInput.value = action.question
  sendMessage()
}

// 获取集群ID
const getClusterId = () => {
  const id = localStorage.getItem('cluster_id')
  if (id) {
    clusterId.value = parseInt(id)
  } else {
    ElMessage.warning('未找到集群ID，请先选择集群')
  }
}

// 格式化时间
const formatTime = (timestamp) => {
  const date = new Date(timestamp)
  return date.toLocaleTimeString('zh-CN', { 
    hour: '2-digit', 
    minute: '2-digit' 
  })
}

// 滚动到底部
const scrollToBottom = () => {
  nextTick(() => {
    if (messagesContainer.value) {
      messagesContainer.value.scrollTop = messagesContainer.value.scrollHeight
    }
  })
}

// 添加消息到聊天记录
const addMessage = (content, type = 'user') => {
  chatHistory.value.push({
    content,
    type,
    timestamp: Date.now()
  })
  scrollToBottom()
}

// 处理键盘事件
const handleKeyDown = (event) => {
  // 直接回车发送消息
  sendMessage()
}

// 复制到剪贴板
const copyToClipboard = async (text) => {
  try {
    await navigator.clipboard.writeText(text)
    ElMessage.success('已复制到剪贴板')
  } catch (err) {
    // 降级方案
    const textArea = document.createElement('textarea')
    textArea.value = text
    document.body.appendChild(textArea)
    textArea.select()
    document.execCommand('copy')
    document.body.removeChild(textArea)
    ElMessage.success('已复制到剪贴板')
  }
}

// 发送消息
const sendMessage = async () => {
  if (!userInput.value.trim() || isLoading.value) return
  
  if (!clusterId.value) {
    ElMessage.error('集群ID未设置，请刷新页面重试')
    return
  }
  
  const question = userInput.value.trim()
  
  // 添加用户消息
  addMessage(question, 'user')
  
  // 清空输入框
  userInput.value = ''
  
  // 设置加载状态
  isLoading.value = true
  
  try {
    // 调用ChatStream API
    const requestData = {
      cluster_id: clusterId.value,
      question: question
    }
    
    const reader = await ChatStream(requestData)
    
    // 添加助手消息占位符
    const assistantMessageIndex = chatHistory.value.length
    addMessage('', 'assistant')
    
    // 处理流式响应
    let assistantResponse = ''
    const decoder = new TextDecoder('utf-8')
    let buffer = ''
    
    while (true) {
      const { done, value } = await reader.read()
      
      if (done) break
      
      // 解码响应数据，使用buffer处理可能被截断的字符
      buffer += decoder.decode(value, { stream: true })
      const lines = buffer.split('\n')
      
      // 保留最后一行（可能不完整）
      buffer = lines.pop() || ''
      
      for (const line of lines) {
        if (line.startsWith('data: ')) {
          const data = line.slice(6).trim()
          if (data && data !== '[DONE]') {
            try {
              const parsed = JSON.parse(data)
              // 处理OpenAI格式的流式响应
              let content = ''
              if (parsed.choices && parsed.choices[0] && parsed.choices[0].delta && parsed.choices[0].delta.content) {
                content = parsed.choices[0].delta.content
              } else if (parsed.content) {
                content = parsed.content
              }
              
              if (content) {
                assistantResponse += content
                // 实时更新助手消息
                chatHistory.value[assistantMessageIndex].content = assistantResponse
                scrollToBottom()
              }
            } catch (e) {
              console.error('解析流式数据错误:', e, 'data:', data)
            }
          }
        }
      }
    }
    
    // 处理剩余的buffer内容
    if (buffer.trim()) {
      if (buffer.startsWith('data: ')) {
        const data = buffer.slice(6).trim()
        if (data && data !== '[DONE]') {
          try {
            const parsed = JSON.parse(data)
            let content = ''
            if (parsed.choices && parsed.choices[0] && parsed.choices[0].delta && parsed.choices[0].delta.content) {
              content = parsed.choices[0].delta.content
            } else if (parsed.content) {
              content = parsed.content
            }
            
            if (content) {
              assistantResponse += content
              chatHistory.value[assistantMessageIndex].content = assistantResponse
              scrollToBottom()
            }
          } catch (e) {
            console.error('解析剩余数据错误:', e, 'data:', data)
          }
        }
      }
    }
    
    // 如果没有收到任何响应，显示默认消息
    if (!assistantResponse) {
      chatHistory.value[assistantMessageIndex].content = '抱歉，我没有收到有效的响应。'
    }
    
  } catch (error) {
    console.error('发送消息失败:', error)
    addMessage('抱歉，发送消息时出现错误，请稍后重试。', 'assistant')
    ElMessage.error('发送消息失败: ' + error.message)
  } finally {
    isLoading.value = false
  }
}

// 组件挂载时获取集群ID
onMounted(() => {
  getClusterId()
})
</script>

<style scoped>
.chat-container {
  display: flex;
  flex-direction: column;
  height: 600px;
  border: 1px solid #e4e7ed;
  border-radius: 8px;
  background: #fff;
}

.chat-messages {
  flex: 1;
  overflow-y: auto;
  padding: 16px;
  background: #f8f9fa;
}

.message-item {
  display: flex;
  margin-bottom: 16px;
  align-items: flex-start;
}

.message-item.user {
  flex-direction: row-reverse;
}

.message-item.user .message-content {
  background: #409eff;
  color: white;
  margin-right: 8px;
}

.message-item.assistant .message-content {
  background: white;
  border: 1px solid #e4e7ed;
  margin-left: 8px;
}

.message-avatar {
  width: 32px;
  height: 32px;
  border-radius: 50%;
  background: #f0f0f0;
  display: flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
}

.message-content {
  max-width: 95%;
  border-radius: 12px;
  padding: 12px 16px;
  position: relative;
  overflow-x: auto;
}

.message-text {
  line-height: 1.5;
  word-wrap: break-word;
  white-space: pre-wrap;
  font-family: 'Monaco', 'Menlo', 'Ubuntu Mono', monospace;
  font-size: 14px;
  overflow-x: auto;
  width: 100%;
}

/* 表格样式 - 强制防止换行 */
.message-text table {
  white-space: nowrap !important;
  border-collapse: collapse;
  width: 100%;
  font-size: 12px;
  table-layout: auto;
  min-width: 600px;
}

.message-text table td,
.message-text table th {
  white-space: nowrap !important;
  overflow: hidden;
  text-overflow: ellipsis;
  padding: 4px 8px;
  border: 1px solid #e4e7ed;
  max-width: 300px;
  min-width: 80px;
}

.message-text table th {
  background-color: #f5f7fa;
  font-weight: bold;
}

/* 对于包含表格数据的文本块，强制防止换行 */
.message-text pre {
  white-space: nowrap !important;
  overflow-x: auto;
  font-family: 'Monaco', 'Menlo', 'Ubuntu Mono', monospace;
  font-size: 12px;
  line-height: 1.4;
  padding: 8px;
  background-color: #f8f9fa;
  border-radius: 4px;
  border: 1px solid #e9ecef;
}

/* 强制所有文本内容不换行 */
.message-text * {
  white-space: nowrap !important;
}

/* 但保持正常的段落换行 */
.message-text p,
.message-text div:not([class]) {
  white-space: pre-wrap !important;
}

/* 特别处理表格头部文本 */
.message-text {
  white-space: pre !important;
  word-break: keep-all !important;
  overflow-wrap: normal !important;
}

/* 特别针对表格行的样式 */
.message-text tr {
  white-space: nowrap !important;
}

.message-text tr td {
  white-space: nowrap !important;
  word-break: keep-all !important;
  overflow: hidden;
  text-overflow: ellipsis;
}

/* 深色模式下的表格样式 */
.dark .message-text table td,
.dark .message-text table th {
  border-color: #444;
}

.dark .message-text table th {
  background-color: #333;
}

.dark .message-text pre {
  background-color: #2a2a2a;
  border-color: #444;
  color: #fff;
}

.message-header {
  display: flex;
  justify-content: flex-end;
  margin-bottom: 8px;
}

.message-time {
  font-size: 12px;
  opacity: 0.6;
  margin-top: 4px;
}

.copy-button {
  opacity: 0;
  transition: opacity 0.2s;
  font-size: 12px;
  padding: 4px;
  height: auto;
  min-height: 24px;
  border-radius: 4px;
}

.message-item.assistant:hover .copy-button {
  opacity: 1;
}

.copy-button:hover {
  background-color: #f5f7fa;
}

.quick-actions-area {
  border-top: 1px solid #e4e7ed;
  padding: 12px 16px;
  background: #fafbfc;
}

.quick-actions-title {
  font-size: 12px;
  color: #909399;
  margin-bottom: 8px;
  font-weight: 500;
}

.quick-actions-buttons {
  display: flex;
  flex-wrap: wrap;
  gap: 2px;
}

.quick-action-btn {
  font-size: 10px;
  height: 20px;
  padding: 0 6px;
  border-radius: 10px;
  transition: all 0.2s;
  margin-right: 3px;
  margin-bottom: 3px;
  white-space: nowrap;
}

.quick-action-btn:hover {
  transform: translateY(-1px);
  box-shadow: 0 2px 8px rgba(64, 158, 255, 0.3);
}

.quick-action-btn .el-icon {
  margin-right: 1px;
  font-size: 10px;
}

.typing-indicator {
  display: flex;
  align-items: center;
  gap: 4px;
}

.typing-indicator span {
  width: 6px;
  height: 6px;
  border-radius: 50%;
  background: #409eff;
  animation: typing 1.4s infinite;
}

.typing-indicator span:nth-child(2) {
  animation-delay: 0.2s;
}

.typing-indicator span:nth-child(3) {
  animation-delay: 0.4s;
}

@keyframes typing {
  0%, 60%, 100% {
    transform: translateY(0);
  }
  30% {
    transform: translateY(-10px);
  }
}

.chat-input-area {
  border-top: 1px solid #e4e7ed;
  padding: 16px;
  background: white;
}

.input-container {
  display: flex;
  gap: 12px;
  align-items: flex-end;
}

.chat-input {
  flex: 1;
}

.send-button {
  height: 40px;
  padding: 0 16px;
}

/* 深色模式适配 */
.dark .chat-container {
  background: #1a1a1a;
  border-color: #333;
}

.dark .chat-messages {
  background: #2a2a2a;
}

.dark .message-item.assistant .message-content {
  background: #333;
  border-color: #444;
  color: #fff;
}

.dark .chat-input-area {
  background: #1a1a1a;
  border-color: #333;
}

.dark .quick-actions-area {
  background: #2a2a2a;
  border-color: #333;
}

.dark .quick-actions-title {
  color: #a8abb2;
}
</style>
