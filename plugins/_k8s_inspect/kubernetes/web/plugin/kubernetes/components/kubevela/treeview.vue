<template>
  <div>
    <el-card>
      <div style="text-align: right;"><el-switch v-model="horizontal" /> 横向</div>
      <div style="height: 800px;">
        <vue3-tree-org
          :data="data"
          :horizontal="horizontal"
          :collapsable="collapsable"
          :label-style="style"
          :only-one-node="onlyOneNode"
          :clone-node-drag="cloneNodeDrag"
          :before-drag-end="beforeDragEnd"
          @on-node-drag="nodeDragMove"
          @on-node-drag-end="nodeDragEnd"
          @on-contextmenu="onMenus"
          @on-expand="onExpand"
          @on-node-dblclick="onNodeDblclick"
          @on-node-click="onNodeClick"
        />
      </div>
    </el-card>
  </div>
</template>

<script>

export default {
  name: 'BaseTree',
}

</script>
<script setup>
import { ElMessage } from 'element-plus'
import { ref } from 'vue'
import { ApplicationVelaqlList } from '@/plugin/kubernetes/api/kubevela/applications'
const props = defineProps({
  data: {
    default: function() {
      return {}
    },
    type: Object
  },
})

const data = ref({})
const TreeViewData = ref([])
const treeDataInit = () => {
  data.value = { 'id': 1, 'label': detailData.value.metadata.name, 'children': [],
    'style': { 'color': 'black', 'text-align': 'center', 'background-repeat': 'no-repeat', 'background-size': '30px 30px', 'background-position': 'left', 'width': '150px',
      'background-image':
          'url(data:image/svg+xml;base64,PHN2ZyBpZD0i5Zu+5bGCXzEiIGRhdGEtbmFtZT0i5Zu+5bGCIDEiIHhtbG5zPSJodHRwOi8vd3d3LnczLm9yZy8yMDAwL3N2ZyIgdmlld0JveD0iMCAwIDEyMi41IDEyMi41Ij48ZGVmcz48c3R5bGU+LmNscy0xe2ZpbGw6IzAwNmZmZjt9PC9zdHlsZT48L2RlZnM+PHRpdGxlPkt1YmVWZWxhIDwvdGl0bGU+PHBhdGggZD0iTTMyLjA1LDk2LjI0YTExLjU4LDExLjU4LDAsMCwwLDEwLDUuNzksMTEuMzQsMTEuMzQsMCwwLDAsNS42Mi0xLjQ3LDYsNiwwLDAsMSw1Ljg5LDBBMTEuMzMsMTEuMzMsMCwwLDAsNTkuMTksMTAyYTExLjUyLDExLjUyLDAsMCwwLDUuMjctMS4yNyw2LjE3LDYuMTcsMCwwLDEsNS43LDAsMTEuNTUsMTEuNTUsMCwwLDAsMTUuMjgtNC41MloiLz48cGF0aCBjbGFzcz0iY2xzLTEiIGQ9Ik05MS41Miw3Ni4yNyw5MS4zOCw3NEM5MCw1MS43OSw4MS43LDMwLjkxLDY4LjA3LDE1LjIzaDB2NjBaIi8+PHBhdGggY2xhc3M9ImNscy0xIiBkPSJNNjQuODYsNzUuMDVWMjcuOEE3OS4yNiw3OS4yNiwwLDAsMCw0Miw3NFoiLz48cGF0aCBkPSJNNDMuODcsNzguMzgsNDIsNzQsMjAuMzUsNzNsNiwxNC4xMmgwYTEwLjE2LDEwLjE2LDAsMCwwLDguNzksNS4wOEExMCwxMCwwLDAsMCw0MCw5MWE3LjMyLDcuMzIsMCwwLDEsNy4xNywwLDEwLDEwLDAsMCwwLDkuNzgsMCw3LjMsNy4zLDAsMCwxLDcuMTcsMCwxMCwxMCwwLDAsMCw5Ljc4LDAsNy4zMiw3LjMyLDAsMCwxLDcuMTcsMEExMCwxMCwwLDAsMCw4Niw5Mi4yM2ExMC4xNiwxMC4xNiwwLDAsMCw4Ljc5LTUuMDhsMi40Ni01TDQ2LDc5Ljg0QTIuMzgsMi4zOCwwLDAsMSw0My44Nyw3OC4zOFoiLz48L3N2Zz4=)' },
  }

  const classifiedResources = {}
  const clusterchildren = []

  TreeViewData.value.forEach(resource => {
    const cluster = resource.cluster
    if (!classifiedResources[cluster]) {
      classifiedResources[cluster] = []
    }
    classifiedResources[cluster].push(resource)
  })

  let ClusterId = 0
  for (const cluster in classifiedResources) {
    if (classifiedResources.hasOwnProperty(cluster)) {
      const resources = classifiedResources[cluster]
      const clusterResources = {
        'id': ClusterId += 1,
        'pid': 1,
        'label': 'Cluster: ' + cluster,
        'style': { 'color': 'black', 'text-align': 'center', 'background-repeat': 'no-repeat', 'background-size': '30px 30px', 'background-position': 'left', 'width': '150px', 'background-image': 'url(data:image/svg+xml;base64,PHN2ZyB0PSIxNjM1NzUxNzc5ODM4IiBjbGFzcz0iaWNvbiIgdmlld0JveD0iMCAwIDEwMjQgMTAyNCIgdmVyc2lvbj0iMS4xIiB4bWxucz0iaHR0cDovL3d3dy53My5vcmcvMjAwMC9zdmciIHAtaWQ9IjE5MDYiPgogICAgPHBhdGggZD0iTTQzNS4zNiA2MTIuMjU2bDAuMjg4IDAuNDE2LTQyLjYyNCAxMDIuOTQ0YTIyMS43OTIgMjIxLjc5MiAwIDAgMS04OC4wMzItMTA5LjI4bC0wLjQ4LTEuNTM2IDEwOS45ODQtMTguNjU2IDAuMTYgMC4yMjRhMTguNzg0IDE4Ljc4NCAwIDAgMSAyMC42MDggMjUuOTg0bDAuMDMyLTAuMTI4eiBtLTM1LjUyLTkwLjgxNmExOC44MTYgMTguODE2IDAgMCAwIDcuNDI0LTMyLjI1NmwtMC4wMzItMC4wMzIgMC4wOTYtMC40OC04My42OC03NC44NDhhMjE3LjA4OCAyMTcuMDg4IDAgMCAwLTMyLjQ4IDExNC44NDhjMCA4LjQ0OCAwLjQ4IDE2LjggMS40MDggMjUuMDI0bC0wLjA5Ni0wLjk5MiAxMDcuMjY0LTMwLjk0NCAwLjA5Ni0wLjM4NHogbTQ4LjgzMi04NC40OGExOC43ODQgMTguNzg0IDAgMCAwIDI5LjgyNC0xNC4zMzZ2LTAuMDMybDAuNDE2LTAuMjI0IDYuNC0xMTEuNzc2YTIxOS4yOTYgMjE5LjI5NiAwIDAgMC0xMjguNDQ4IDYxLjU2OGwwLjAzMi0wLjAzMiA5MS42MTYgNjQuOTkyIDAuMTYtMC4wOTZ6IG0zMi40NDggMTE3LjMxMmwzMC44NDggMTQuODggMzAuODE2LTE0LjgxNiA3LjY4LTMzLjI4LTIxLjM0NC0yNi41OTJoLTM0LjMwNGwtMjEuMzQ0IDI2LjU5MiA3LjY0OCAzMy4yNDh6IG02NC0xMzIuMDMyYTE4Ljc4NCAxOC43ODQgMCAwIDAgMjkuOTIgMTQuMzA0bC0wLjA2NCAwLjAzMiAwLjM1MiAwLjEyOCA5MS4wNC02NC41NDRhMjIwLjY3MiAyMjAuNjcyIDAgMCAwLTEyNi41Ni02MS40MDhsLTEuMDg4LTAuMTI4IDYuMzA0IDExMS41ODQgMC4wOTYgMC4wMzJ6IG00NjQuMDMyIDI1NC43MmwtMjQ2LjMwNCAzMDYuNGE2OC4xNiA2OC4xNiAwIDAgMS01My4wODggMjUuMzQ0aC0wLjE2bC0zOTUuMTM2IDAuMTI4aC0wLjA2NGE2OC4wOTYgNjguMDk2IDAgMCAxLTUzLjA1Ni0yNS4zMTJsLTAuMDk2LTAuMTI4LTI0Ni40MzItMzA2LjMzNmE2Ny4xMDQgNjcuMTA0IDAgMCAxLTEyLjk5Mi01Ny42MzJsLTAuMDk2IDAuNDQ4IDg3LjkwNC0zODIuMDhBNjcuNTIgNjcuNTIgMCAwIDEgMTI2LjQ2NCAxOTJMNDgyLjQgMjEuODg4YzguNjQtNC4yMjQgMTguODE2LTYuNjg4IDI5LjUzNi02LjY4OHMyMC44OTYgMi40NjQgMjkuOTUyIDYuODhsLTAuNDE2LTAuMTkyIDM1Ni4wNjQgMTcwLjAxNmE2Ny41NTIgNjcuNTUyIDAgMCAxIDM2LjgzMiA0NS43OTJsODcuOTY4IDM4Mi4wOGE2Ny4yIDY3LjIgMCAwIDEtMTMuMTUyIDU3LjE4NHogbS0xNDAuMzItODcuNzc2Yy0xLjc5Mi0wLjQxNi00LjM4NC0xLjEyLTYuMTc2LTEuNDQtNy40MjQtMS40MDgtMTMuNDQtMS4wNTYtMjAuNDQ4LTEuNjMyLTE0Ljk0NC0xLjU2OC0yNy4yMzItMi44NDgtMzguMTc2LTYuMzA0LTQuNDgtMS42OTYtNy42OC03LjA0LTkuMjE2LTkuMjE2bC04LjU3Ni0yLjUyOGEyNzUuOTM2IDI3NS45MzYgMCAwIDAtNC44MzItMTAxLjM3NmwwLjM4NCAxLjg1NmEyNzYuMzg0IDI3Ni4zODQgMCAwIDAtNDAuNTQ0LTkzLjI0OGwwLjYwOCAwLjk2YzIuMjA4LTIuMDE2IDYuNC01LjY2NCA3LjU1Mi02Ljc4NCAwLjM1Mi0zLjg0IDAuMDMyLTcuODA4IDQtMTIuMDMyIDguNDE2LTcuOTA0IDE4Ljk0NC0xNC40MzIgMzEuNzEyLTIyLjI3MiA2LjA0OC0zLjU4NCAxMS42NDgtNS44NTYgMTcuNjk2LTEwLjMzNiAxLjM3Ni0xLjAyNCAzLjIzMi0yLjY1NiA0LjcwNC0zLjgwOCAxMC4yNC04LjE2IDEyLjU3Ni0yMi4xNzYgNS4yNDgtMzEuMzkycy0yMS42LTEwLjA4LTMxLjc3Ni0xLjkyYy0xLjQ0IDEuMTUyLTMuNDI0IDIuNjU2LTQuNzM2IDMuNzQ0LTUuNzI4IDQuOTYtOS4yNDggOS44MjQtMTQuMDggMTQuOTQ0LTEwLjQ5NiAxMC42NTYtMTkuMiAxOS41NTItMjguNzA0IDI1Ljk4NC00LjEyOCAyLjQtMTAuMjA4IDEuNTY4LTEyLjkyOCAxLjQwOGwtOC4wOTYgNS43NmEyNzguNzIgMjc4LjcyIDAgMCAwLTE3NS43NzYtODUuMzc2bC0xLjEyLTAuMDk2LTAuNTEyLTkuNTA0Yy0yLjc4NC0yLjY1Ni02LjExMi00Ljg5Ni02Ljk0NC0xMC42NTYtMC45MjgtMTEuNDI0IDAuNjQtMjMuNzc2IDIuNDMyLTM4LjYyNCAwLjk5Mi02Ljk0NCAyLjU5Mi0xMi43MDQgMi45MTItMjAuMjU2IDAuMDMyLTEuNjk2LTAuMDMyLTQuMjI0LTAuMDMyLTYuMDQ4IDAtMTMuMDU2LTkuNTY4LTIzLjY4LTIxLjM0NC0yMy42OC0xMS43NDQgMC0yMS4yOCAxMC42MjQtMjEuMjggMjMuNjhsMC4wMzIgMC42MDhjMCAxLjc2LTAuMDk2IDMuOTM2IDAgNS40NzIgMC4yNTYgNy41NTIgMS44ODggMTMuMzEyIDIuODQ4IDIwLjI1NiAxLjc5MiAxNC44NDggMy4zMjggMjcuMTY4IDIuNCAzOC42NTZhMjMuMzYgMjMuMzYgMCAwIDEtNi44OCAxMC45NzZsLTAuMDMyIDAuMDMyLTAuNTEyIDguOTkyYTI3NC4wOCAyNzQuMDggMCAwIDAtMTc3LjYzMiA4NS4zMTJsLTAuMTI4IDAuMTI4YTMzOC45NzYgMzM4Ljk3NiAwIDAgMS04LjY3Mi02LjE0NGwwLjk5MiAwLjcwNGMtMy44NCAwLjUxMi03LjY4IDEuNjk2LTEyLjY3Mi0xLjI0OC05LjUwNC02LjQtMTguMjA4LTE1LjI2NC0yOC43MDQtMjUuOTUyLTQuODMyLTUuMTItOC4zMi05Ljk4NC0xNC4wNDgtMTQuODhhOTcuNzI4IDk3LjcyOCAwIDAgMC00LjczNi0zLjc0NCAyNS4xODQgMjUuMTg0IDAgMCAwLTE0Ljc4NC01LjYzMmgtMC4wNjRsLTEuMDg4LTAuMDMyYTIwLjQ4IDIwLjQ4IDAgMCAwLTE1Ljg3MiA3LjQ4OGwtMC4wMzIgMC4wMzJjLTcuMzI4IDkuMjE2LTQuOTkyIDIzLjI5NiA1LjI0OCAzMS40NTZsMC4yODggMC4yMjQgNC40NDggMy41NTJjNi4wNDggNC40OCAxMS42MTYgNi43ODQgMTcuNjY0IDEwLjMzNiAxMi43NjggNy45MDQgMjMuMjk2IDE0LjQzMiAzMS43MTIgMjIuMjcyIDMuMjMyIDMuNDg4IDMuODQgOS42MzIgNC4yNTYgMTIuMjg4bDYuODE2IDYuMTEyYTI3My4yOCAyNzMuMjggMCAwIDAtNDYuMzM2IDE1Mi45OTJjMCAxMy44NTYgMS4wMjQgMjcuNDg4IDMuMDA4IDQwLjhsLTAuMTkyLTEuNTA0LTguODY0IDIuNTZjLTIuMzM2IDMuMDcyLTUuNjY0IDcuODQtOS4xODQgOS4yNDgtMTAuOTc2IDMuNDU2LTIzLjI5NiA0LjcwNC0zOC4xNzYgNi4yNzItNy4wMDggMC42MDgtMTMuMDI0IDAuMjU2LTIwLjQ4IDEuNjY0LTEuNTY4IDAuMjg4LTMuODQgMC44NjQtNS42NjQgMS4yOGwtMC4xNiAwLjA5Ni0wLjI4OCAwLjA5NmMtMTIuNTc2IDMuMDQtMjAuNjQgMTQuNTkyLTE4LjA0OCAyNS45NTIgMi41OTIgMTEuMzkyIDE0Ljg4IDE4LjMwNCAyNy41MiAxNS41ODRsMC4yODgtMC4wMzIgMC40MTYtMC4xMjggNS41MDQtMS4yNDhjNy4yNjQtMS45NTIgMTIuNTQ0LTQuODMyIDE5LjEwNC03LjMyOCAxNC4wOC01LjAyNCAyNS43Ni05LjI0OCAzNy4xMi0xMC45MTIgNC43NjgtMC4zODQgOS44MjQgMi45NDQgMTIuMjg4IDQuMzJsOS4yNDgtMS41NjhhMjc4LjkxMiAyNzguOTEyIDAgMCAwIDEyMS42MzIgMTUyLjczNmwxLjI0OCAwLjcwNC0zLjg0IDkuMzEyYzEuNDA4IDMuNTg0IDIuOTQ0IDguNDggMS44ODggMTIuMDMyLTQuMTI4IDEwLjc1Mi0xMS4yMzIgMjIuMDQ4LTE5LjI5NiAzNC42ODgtMy44NzIgNS43OTItNy45MDQgMTAuMzM2LTExLjQyNCAxNy4wMjQtMC44NjQgMS41NjgtMS45MiA0LjA2NC0yLjcyIDUuNzI4LTUuNDcyIDExLjc0NC0xLjQ0IDI1LjIxNiA5LjA4OCAzMC4zMDQgMTAuNTkyIDUuMTIgMjMuNzEyLTAuMjg4IDI5LjQ0LTEyLjAzMnYtMC4wOTZjMC44NjQtMS42NjQgMS45NTItMy44NCAyLjY1Ni01LjQwOCAyLjk3Ni02LjkxMiA0LTEyLjgzMiA2LjE0NC0xOS41NTIgNS42MzItMTQuMTc2IDguNzM2LTI5LjAyNCAxNi41MTItMzguMjcyIDIuMTQ0LTIuNTYgNS41MzYtMy40ODggOS4xODQtNC40OGw0LjgzMi04LjczNmEyNzAuNTkyIDI3MC41OTIgMCAwIDAgOTkuMDA4IDE4LjQgMjc0LjU2IDI3NC41NiAwIDAgMCA5OS41Mi0xOC41MjhsLTEuODg4IDAuNjQgNC41MTIgOC4xOTJjMy42OCAxLjE4NCA3LjY4IDEuNzkyIDEwLjkxMiA2LjYyNCA1Ljc5MiA5Ljg4OCA5Ljc2IDIxLjYzMiAxNC41OTIgMzUuODQgMi4xNDQgNi42NTYgMy4xNjggMTIuNTc2IDYuMTc2IDE5LjQ4OCAwLjY3MiAxLjU2OCAxLjgyNCAzLjg0IDIuNjU2IDUuNTA0IDUuNjY0IDExLjc3NiAxOC44NDggMTcuMTUyIDI5LjQ0IDEyLjAzMiAxMC41MjgtNS4wMjQgMTQuNTYtMTguNTYgOS4wODgtMzAuMzA0LTAuODY0LTEuNjY0LTEuOTItNC4wOTYtMi43ODQtNS43MjgtMy41NTItNi42NTYtNy41NTItMTEuMTM2LTExLjQyNC0xNi45OTItOC4wOTYtMTIuNjQtMTQuNzUyLTIzLjA3Mi0xOC45MTItMzMuODI0LTEuNjk2LTUuNTM2IDAuMjg4LTguOTYgMS42MzItMTIuNTQ0LTAuNzY4LTAuOTI4LTIuNTI4LTYuMTQ0LTMuNTUyLTguNjA4YTI3OC44OCAyNzguODggMCAwIDAgMTIyLjMzNi0xNTIuNTc2bDAuNTQ0LTEuOTUyYzIuNzIgMC40MTYgNy41MiAxLjI4IDkuMDg4IDEuNjMyIDMuMi0yLjE0NCA2LjE0NC00Ljg2NCAxMS45MzYtNC40NDggMTEuMzYgMS42NjQgMjMuMDQgNS44ODggMzcuMTIgMTAuOTEyIDYuNTYgMi41NiAxMS44MDggNS40NzIgMTkuMTA0IDcuMzkyIDEuNTM2IDAuNDE2IDMuNzQ0IDAuOCA1LjUzNiAxLjE4NGwwLjM4NCAwLjEyOCAwLjI4OCAwLjAzMmMxMi42NzIgMi43MiAyNC45MjgtNC4xOTIgMjcuNTItMTUuNTg0IDIuNTYtMTEuMzYtNS40NzItMjIuOTEyLTE4LjA0OC0yNS45NTJ6IG0tMTY5LjA4OC0xNzUuMjY0bC04My4yIDc0LjQ5NnYwLjIyNGExOC43ODQgMTguNzg0IDAgMCAwIDcuMjY0IDMyLjI1NmwwLjEyOCAwLjAzMiAwLjEyOCAwLjQxNiAxMDcuNzc2IDMxLjA3MmEyMjIuNCAyMjIuNCAwIDAgMC00Ljg5Ni03Mi45NmwwLjI4OCAxLjUwNGEyMjIuMzM2IDIyMi4zMzYgMCAwIDAtMjguMDMyLTY4LjA2NGwwLjU0NCAwLjkyOHogbS0xNzEuMjMyIDIyNy4yYTE4LjY1NiAxOC42NTYgMCAwIDAtMTYuNDgtOS45MmwtMC44IDAuMDMyaDAuMDMyYTE4LjcyIDE4LjcyIDAgMCAwLTE1LjgwOCA5Ljg1NmwtMC4wNjQgMC4wOTZoLTAuMDk2bC01NC4xMTIgOTcuNzkyYTIxNS43MTIgMjE1LjcxMiAwIDAgMCA3MS4xMzYgMTEuODA4YzI1LjM0NCAwIDQ5LjY2NC00LjI4OCA3Mi4zMi0xMi4xMjhsLTEuNTM2IDAuNDgtNTQuMTc2LTk3Ljk1MmgtMC40MTZ6IG04MC41NDQtNTUuMTY4YTE4Ljc4NCAxOC43ODQgMCAwIDAtMTEuNjE2IDEuNTY4bDAuMDk2LTAuMDY0YTE4LjgxNiAxOC44MTYgMCAwIDAtOS4wODggMjQuNTEybC0wLjAzMi0wLjEyOC0wLjEyOCAwLjE2IDQzLjEwNCAxMDQuMDMyYTIyMC44IDIyMC44IDAgMCAwIDg4LjI4OC0xMTAuMDQ4bDAuNDgtMS41MzYtMTEwLjk0NC0xOC43ODQtMC4xNiAwLjIyNHoiIGZpbGw9IiMzMjZDRTUiIHAtaWQ9IjE5MDciPjwvcGF0aD4KPC9zdmc+)' },
        'children': []
      }

      let clusterChildrenId = 0
      const clusterResourceschildren = []
      for (const r in resources) {
        const clusterResourceschildrenTmp = {
          'id': clusterChildrenId += 1,
          'pid': 2,
          'label': 'Kind: ' + resources[r].kind + ' Name: ' + resources[r].name + ' Namespace: ' + resources[r].namespace,
          'style': { 'color': 'black', 'width': 'auto' },
          'children': []
        }

        let clusterChildrenId2 = 0
        if (resources[r].resourceTree) {
          for (const b in resources[r].resourceTree.leafNodes) {
            const clusterResourceschildrenTmp2 = {
              'id': clusterChildrenId2 += 1,
              'pid': 3,
              'label': 'Kind: ' + resources[r].resourceTree.leafNodes[b].kind + ' Name: ' + resources[r].resourceTree.leafNodes[b].name + ' Namespace: ' + resources[r].resourceTree.leafNodes[b].namespace,
              'style': { 'color': 'black', 'width': 'auto' },
              'children': []
            }

            let clusterChildrenId3 = 0
            if (resources[r].resourceTree.leafNodes[b].leafNodes) {
              for (const c in resources[r].resourceTree.leafNodes[b].leafNodes) {
                const clusterResourceschildrenTmp3 = {
                  'id': clusterChildrenId3 += 1,
                  'pid': 4,
                  'label': 'Kind: ' + resources[r].resourceTree.leafNodes[b].leafNodes[c].kind + ' Name: ' + resources[r].resourceTree.leafNodes[b].leafNodes[c].name + ' Namespace: ' + resources[r].resourceTree.leafNodes[b].leafNodes[c].namespace,
                  'style': { 'color': 'black', 'width': 'auto' },
                  'children': []
                }
                clusterResourceschildrenTmp2.children.push(clusterResourceschildrenTmp3)
              }
            }

            clusterResourceschildrenTmp.children.push(clusterResourceschildrenTmp2)
          }
        }
        clusterResourceschildren.push(clusterResourceschildrenTmp)
      }

      clusterResources.children = clusterResourceschildren
      clusterchildren.push(clusterResources)
    }
  }

  data.value.children = clusterchildren
}

const detailData = ref({})
const velaql = ref('')

const GetApplicationResourceTreeView = async() => {
  velaql.value = 'application-resource-tree-view{appNs=' + detailData.value.metadata.namespace + ',appName=' + detailData.value.metadata.name + '}.status'
  const res = await ApplicationVelaqlList(detailData.value.cluster_id, velaql.value)
  if (res.code === 0) {
    TreeViewData.value = res.data.resources
  }
}

// 数据初始化
const detailInitData = async() => {
  detailData.value = props.data
  await GetApplicationResourceTreeView()
  treeDataInit()
}

detailInitData()

const horizontal = ref(true)
const collapsable = ref(false)
const onlyOneNode = ref(false)
const cloneNodeDrag = ref(true)

const style = ref({
  background: '#fff',
  color: '#5e6d82',
})

const toggleExpand = (data, val) => {
  if (Array.isArray(data)) {
    data.forEach((item) => {
      item.expand = val
      if (item.children) {
        toggleExpand(item.children, val)
      }
    })
  } else {
    data.expand = val
    if (data.children) {
      toggleExpand(data.children, val)
    }
  }
}

const onMenus = ({ node, command }) => {
  console.log(node, command)
}

const onExpand = (e, data) => {
  console.log(e, data)
}

const nodeDragMove = (data) => {
  console.log(data)
}

const beforeDragEnd = (node, targetNode) => {
  return new Promise((resolve, reject) => {
    if (!targetNode) reject()
    if (node.id === targetNode.id) {
      reject()
    } else {
      resolve()
    }
  })
}

const nodeDragEnd = (data, isSelf) => {
  console.log(data, isSelf)
}

const onNodeDblclick = () => {
  console.log('onNodeDblclick')
}

const onNodeClick = (e, data) => {
  ElMessage.info(data.label)
}
</script>
<style scoped>
</style>

