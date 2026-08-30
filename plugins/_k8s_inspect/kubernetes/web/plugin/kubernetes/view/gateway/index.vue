<template>
  <div class="container">
    <ClusterNamespace />
    <el-tabs style="width:100%;">
      <el-tab-pane label="GatewayClass">
        <!--子组件-->
        <GatewayClass />
      </el-tab-pane>
      <el-tab-pane label="Gateway">
        <!--子组件-->
        <Gateway />
      </el-tab-pane>
      <el-tab-pane label="HTTPRoute">
        <!--子组件-->
        <HTTPRoute />
      </el-tab-pane>
      <el-tab-pane label="GRPCRoute">
        <GRPCRoute />
      </el-tab-pane>
      <el-tab-pane label="TCPRoute">
        <TCPRoute />
      </el-tab-pane>
      <el-tab-pane label="TLSRoute">
        <TLSRoute />
      </el-tab-pane>
      <el-tab-pane label="UDPRoute">
        <UDPRoute />
      </el-tab-pane>
    </el-tabs>
  </div>
</template>
<script>
import Gateway from './Gateway/index.vue'
import GatewayClass from './GatewayClass/index.vue'
import HTTPRoute from './HTTPRoute/index.vue'
import GRPCRoute from './GRPCRoute/index.vue'
import TCPRoute from './TCPRoute/index.vue'
import TLSRoute from './TLSRoute/index.vue'
import UDPRoute from './UDPRoute/index.vue'
import ClusterNamespace from '@/plugin/kubernetes/components/select/index.vue'
export default {
  name: 'Network',
  components: {
    Gateway,
    GRPCRoute,
    TCPRoute,
    TLSRoute,
    UDPRoute,
    GatewayClass,
    HTTPRoute,
    ClusterNamespace
  }
}
</script>
