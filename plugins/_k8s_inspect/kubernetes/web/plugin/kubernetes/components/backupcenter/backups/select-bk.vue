<template>
  <el-select v-model="backups" filterable @change="handleNamespaceChange">
    <el-option
      v-for="item in backups_list"
      :key="item.name"
      :label="item.name"
      :value="item.name"
    />
  </el-select>
</template>

<script>
export default {
  name: 'NamespaceSelectBlock',
  emits: {
    enter: (value) => {
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
import { ref } from 'vue'
import {BackupsList} from "@/plugin/kubernetes/api/velero/backups";
const emit = defineEmits(['enter'])
const props = defineProps({
  backups: {
    default: function() {
      return ''
    },
    type: String
  },
  form: {
    default: function() {
      return {}
    },
    type: Object
  },
  cluster: {
    default: function() {
      return 0
    },
    type: Number
  },
})

// 命名空间
const backups_list = ref([])
const getNamespaceData = async(cluster_id) => {
  const res = await BackupsList(cluster_id, 0, 0, props.namespace, '', '')
  if (res.code === 0) {
    backups_list.value = []
    for (const bk of res.data.items) {
      const item = {
        id: bk.metadata.name,
        name: bk.metadata.name,
      }
      backups_list.value.push(item)
    }
  }
}

// 数据初始化
const backups = ref('')
const cluster_id = ref(0)
const FormInitData = () => {
  backups.value = props.backups
  cluster_id.value = props.cluster
  getNamespaceData(cluster_id.value)
}

FormInitData()

const handleNamespaceChange = (value) => {
  emit('enter', value)
}
</script>
<style scoped>

</style>
