<template>
  <el-select v-model="namespaceStr" multiple filterable @change="handleNamespaceChange">
    <el-option
      v-for="item in namespace_list"
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
import { getClusterUserNamespace } from '@/plugin/kubernetes/api/namespaces'
const emit = defineEmits(['enter'])
const props = defineProps({
  namespaces: {
    default: function() {
      return []
    },
    type: Array
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
const defaultNamespace  = ref({ 'id': '*', 'name': '*' })
const getNamespaceData = async(cluster_id) => {
  const res = await getClusterUserNamespace({ 'id': cluster_id })
  if (res.code === 0) {
    namespace_list.value = []
    for (const ns of res.data.namespaces) {
      const item = {
        id: ns,
        name: ns,
      }
      namespace_list.value.push(item)
    }

    namespace_list.value.push(defaultNamespace.value)
  }
}

// 数据初始化
const namespaceStr = ref('')
const cluster_id = ref(0)
const namespace_list = ref([])
const FormInitData = () => {
  namespaceStr.value = props.namespaces
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
