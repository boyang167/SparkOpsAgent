<template>
  <div>
    <el-card class="el-card">
      <el-form ref="FormBlock" :model="formData" :rules="rules" label-width="120px" :disabled="readOnly">
        <!-- 基础信息 -->
        <el-divider content-position="left">基础信息</el-divider>
        <el-row :gutter="24">
          <el-col :span="8">
            <el-form-item label="名称" prop="metadata.name">
              <form-item-block
                  v-model="formData.metadata.name"
                  placeholder="请输入名称"
                  item-type="input"
                  style="width: 100%"
              />
            </el-form-item>
          </el-col>
          <el-col :span="8">
            <el-form-item label="命名空间" prop="metadata.namespace">
              <NsSelect :cluster="cluster_id" v-model:namespace="formData.metadata.namespace" />
            </el-form-item>
          </el-col>
        </el-row>

        <!-- 标签和注解 -->
        <el-row :gutter="24">
          <el-col :span="12">
            <el-form-item label="标签">
              <el-table :data="getObjectKeys(formData.metadata.labels).map(key => ({key, value: formData.metadata.labels[key]}))" style="width: 100%" size="small">
                <el-table-column prop="key" label="键" width="120">
                  <template #default="scope">
                    <el-input v-model="scope.row.key" @blur="updateKeyValue(formData.metadata.labels, scope.$index, scope.row.key, scope.row.value)" size="small" />
                  </template>
                </el-table-column>
                <el-table-column prop="value" label="值">
                  <template #default="scope">
                    <el-input v-model="scope.row.value" @blur="updateKeyValue(formData.metadata.labels, scope.$index, scope.row.key, scope.row.value)" size="small" />
                  </template>
                </el-table-column>
                <el-table-column label="操作" width="80">
                  <template #default="scope">
                    <el-button type="danger" link size="small" @click="removeKeyValue(formData.metadata.labels, scope.row.key)">删除</el-button>
                  </template>
                </el-table-column>
              </el-table>
              <div style="margin-top: 10px;">
                <el-input v-model="formData.metadata.labels.newKey" placeholder="键" style="width: 120px; margin-right: 5px;" size="small" />
                <el-input v-model="formData.metadata.labels.newValue" placeholder="值" style="width: 150px; margin-right: 5px;" size="small" />
                <el-button type="primary" size="small" @click="addKeyValue(formData.metadata.labels)">添加</el-button>
              </div>
            </el-form-item>
          </el-col>
          <el-col :span="12">
            <el-form-item label="注解">
              <el-table :data="getObjectKeys(formData.metadata.annotations).map(key => ({key, value: formData.metadata.annotations[key]}))" style="width: 100%" size="small">
                <el-table-column prop="key" label="键" width="120">
                  <template #default="scope">
                    <el-input v-model="scope.row.key" @blur="updateKeyValue(formData.metadata.annotations, scope.$index, scope.row.key, scope.row.value)" size="small" />
                  </template>
                </el-table-column>
                <el-table-column prop="value" label="值">
                  <template #default="scope">
                    <el-input v-model="scope.row.value" @blur="updateKeyValue(formData.metadata.annotations, scope.$index, scope.row.key, scope.row.value)" size="small" />
                  </template>
                </el-table-column>
                <el-table-column label="操作" width="80">
                  <template #default="scope">
                    <el-button type="danger" link size="small" @click="removeKeyValue(formData.metadata.annotations, scope.row.key)">删除</el-button>
                  </template>
                </el-table-column>
              </el-table>
              <div style="margin-top: 10px;">
                <el-input v-model="formData.metadata.annotations.newKey" placeholder="键" style="width: 120px; margin-right: 5px;" size="small" />
                <el-input v-model="formData.metadata.annotations.newValue" placeholder="值" style="width: 150px; margin-right: 5px;" size="small" />
                <el-button type="primary" size="small" @click="addKeyValue(formData.metadata.annotations)">添加</el-button>
              </div>
            </el-form-item>
          </el-col>
        </el-row>

        <!-- NetworkPolicy 配置 -->
        <el-divider content-position="left">网络策略配置</el-divider>

        <!-- PodSelector -->
        <el-form-item label="PodSelector">
          <el-checkbox v-model="formData.spec.podSelector.isEmpty" label="匹配所有 Pod" />
          <div v-if="!formData.spec.podSelector.isEmpty">
            <el-table :data="getObjectKeys(formData.spec.podSelector.matchLabels).map(key => ({key, value: formData.spec.podSelector.matchLabels[key]}))" style="width: 100%" size="small">
              <el-table-column prop="key" label="键" width="120">
                <template #default="scope">
                  <el-input v-model="scope.row.key" @blur="updateKeyValue(formData.spec.podSelector.matchLabels, scope.$index, scope.row.key, scope.row.value)" size="small" />
                </template>
              </el-table-column>
              <el-table-column prop="value" label="值">
                <template #default="scope">
                  <el-input v-model="scope.row.value" @blur="updateKeyValue(formData.spec.podSelector.matchLabels, scope.$index, scope.row.key, scope.row.value)" size="small" />
                </template>
              </el-table-column>
              <el-table-column label="操作" width="80">
                <template #default="scope">
                  <el-button type="danger" link size="small" @click="removeKeyValue(formData.spec.podSelector.matchLabels, scope.row.key)">删除</el-button>
                </template>
              </el-table-column>
            </el-table>
            <div style="margin-top: 10px;">
              <el-input v-model="formData.spec.podSelector.matchLabels.newKey" placeholder="键" style="width: 120px; margin-right: 5px;" size="small" />
              <el-input v-model="formData.spec.podSelector.matchLabels.newValue" placeholder="值" style="width: 150px; margin-right: 5px;" size="small" />
              <el-button type="primary" size="small" @click="addKeyValue(formData.spec.podSelector.matchLabels)">添加</el-button>
            </div>
          </div>
          <div v-if="!formData.spec.podSelector.isEmpty" style="margin-top: 10px;">
            <el-button type="primary" size="small" @click="togglePodSelectorType">
              {{ formData.spec.podSelector.selectorType === 'matchLabels' ? '切换到 MatchExpressions' : '切换到 MatchLabels' }}
            </el-button>
            <div v-if="formData.spec.podSelector.selectorType === 'matchExpressions'" style="margin-top: 10px;">
              <div v-for="(expr, index) in formData.spec.podSelector.matchExpressions" :key="'pod-expr-' + index">
                <el-input v-model="expr.key" placeholder="Key" style="width: 150px; margin-right: 5px;" />
                <el-select v-model="expr.operator" placeholder="Operator" style="width: 120px; margin-right: 5px;">
                  <el-option label="In" value="In" />
                  <el-option label="NotIn" value="NotIn" />
                  <el-option label="Exists" value="Exists" />
                  <el-option label="DoesNotExist" value="DoesNotExist" />
                </el-select>
                <el-input
                    v-if="expr.operator === 'In' || expr.operator === 'NotIn'"
                    v-model="expr.values"
                    placeholder="Values (逗号分隔)"
                    style="width: 200px; margin-right: 5px;"
                />
                <el-button type="danger" link icon="delete" size="small" @click="formData.spec.podSelector.matchExpressions.splice(index, 1)">删除</el-button>
              </div>
              <el-button
                  size="small" type="primary" link icon="plus"
                  @click="formData.spec.podSelector.matchExpressions.push({ key: '', operator: '', values: '' })"
              >
                新增表达式
              </el-button>
            </div>
          </div>
        </el-form-item>

        <!-- PolicyTypes -->
        <el-form-item label="策略类型" prop="spec.policyTypes">
          <el-checkbox-group v-model="formData.spec.policyTypes">
            <el-checkbox label="Ingress">Ingress</el-checkbox>
            <el-checkbox label="Egress">Egress</el-checkbox>
          </el-checkbox-group>
        </el-form-item>

        <!-- Ingress -->
        <el-divider content-position="left">Ingress 规则</el-divider>
        <el-button
            size="small"
            type="primary"
            @click="addIngressRule"
            :disabled="readOnly"
        >
          新增 Ingress
        </el-button>
        <div v-for="(rule, rIndex) in formData.spec.ingress" :key="'ingress-' + rIndex" class="rule-block">
          <el-card shadow="hover" style="margin-top: 10px;">
            <div v-for="(from, fIndex) in rule.from" :key="'from-' + fIndex" style="margin-bottom: 8px;">
              <el-select v-model="from.type" placeholder="选择类型" style="width: 150px" :disabled="readOnly">
                <el-option label="ipBlock" value="ipBlock" />
                <el-option label="namespaceSelector" value="namespaceSelector" />
                <el-option label="podSelector" value="podSelector" />
              </el-select>

              <template v-if="from.type === 'ipBlock' && from.ipBlock">
                <el-input
                    v-model="from.ipBlock.cidr"
                    placeholder="CIDR (如 10.0.0.0/24)"
                    style="width: 200px; margin-left: 10px;"
                    :disabled="readOnly"
                />
                <el-input
                    v-model="from.ipBlock.except"
                    placeholder="Except (如 10.0.0.1/32, 用逗号分隔)"
                    style="width: 200px; margin-left: 10px;"
                    :disabled="readOnly"
                />
              </template>

              <template v-else-if="from.type === 'namespaceSelector' && from.namespaceSelector">
                <el-checkbox v-model="from.namespaceSelector.isEmpty" label="匹配所有 Namespace" :disabled="readOnly" />
                <div v-if="!from.namespaceSelector.isEmpty">
                  <el-table :data="getObjectKeys(from.namespaceSelector.matchLabels).map(key => ({key, value: from.namespaceSelector.matchLabels[key]}))" style="width: 100%" size="small">
                    <el-table-column prop="key" label="键" width="120">
                      <template #default="scope">
                        <el-input v-model="scope.row.key" @blur="updateKeyValue(from.namespaceSelector.matchLabels, scope.$index, scope.row.key, scope.row.value)" size="small" />
                      </template>
                    </el-table-column>
                    <el-table-column prop="value" label="值">
                      <template #default="scope">
                        <el-input v-model="scope.row.value" @blur="updateKeyValue(from.namespaceSelector.matchLabels, scope.$index, scope.row.key, scope.row.value)" size="small" />
                      </template>
                    </el-table-column>
                    <el-table-column label="操作" width="80">
                      <template #default="scope">
                        <el-button type="danger" link size="small" @click="removeKeyValue(from.namespaceSelector.matchLabels, scope.row.key)">删除</el-button>
                      </template>
                    </el-table-column>
                  </el-table>
                  <div style="margin-top: 10px;">
                    <el-input v-model="from.namespaceSelector.matchLabels.newKey" placeholder="键" style="width: 120px; margin-right: 5px;" size="small" />
                    <el-input v-model="from.namespaceSelector.matchLabels.newValue" placeholder="值" style="width: 150px; margin-right: 5px;" size="small" />
                    <el-button type="primary" size="small" @click="addKeyValue(from.namespaceSelector.matchLabels)">添加</el-button>
                  </div>
                </div>
                <div v-if="!from.namespaceSelector.isEmpty" style="margin-top: 10px;">
                  <el-button type="primary" size="small" @click="toggleSelectorType(from, 'namespaceSelector')">
                    {{ from.namespaceSelector.selectorType === 'matchLabels' ? '切换到 MatchExpressions' : '切换到 MatchLabels' }}
                  </el-button>
                  <div v-if="from.namespaceSelector.selectorType === 'matchExpressions'" style="margin-top: 10px;">
                    <div v-for="(expr, index) in from.namespaceSelector.matchExpressions" :key="'ns-expr-' + index">
                      <el-input v-model="expr.key" placeholder="Key" style="width: 150px; margin-right: 5px;" />
                      <el-select v-model="expr.operator" placeholder="Operator" style="width: 120px; margin-right: 5px;">
                        <el-option label="In" value="In" />
                        <el-option label="NotIn" value="NotIn" />
                        <el-option label="Exists" value="Exists" />
                        <el-option label="DoesNotExist" value="DoesNotExist" />
                      </el-select>
                      <el-input
                          v-if="expr.operator === 'In' || expr.operator === 'NotIn'"
                          v-model="expr.values"
                          placeholder="Values (逗号分隔)"
                          style="width: 200px; margin-right: 5px;"
                      />
                      <el-button
                          type="danger" link icon="delete" size="small"
                          @click="from.namespaceSelector.matchExpressions.splice(index, 1)"
                      >
                        删除
                      </el-button>
                    </div>
                    <el-button
                        size="small" type="primary" link icon="plus"
                        @click="from.namespaceSelector.matchExpressions.push({ key: '', operator: '', values: '' })"
                    >
                      新增表达式
                    </el-button>
                  </div>
                </div>
              </template>

              <template v-else-if="from.type === 'podSelector' && from.podSelector">
                <el-checkbox v-model="from.podSelector.isEmpty" label="匹配所有 Pod" :disabled="readOnly" />
                <div v-if="!from.podSelector.isEmpty">
                  <el-table :data="getObjectKeys(from.podSelector.matchLabels).map(key => ({key, value: from.podSelector.matchLabels[key]}))" style="width: 100%" size="small">
                    <el-table-column prop="key" label="键" width="120">
                      <template #default="scope">
                        <el-input v-model="scope.row.key" @blur="updateKeyValue(from.podSelector.matchLabels, scope.$index, scope.row.key, scope.row.value)" size="small" />
                      </template>
                    </el-table-column>
                    <el-table-column prop="value" label="值">
                      <template #default="scope">
                        <el-input v-model="scope.row.value" @blur="updateKeyValue(from.podSelector.matchLabels, scope.$index, scope.row.key, scope.row.value)" size="small" />
                      </template>
                    </el-table-column>
                    <el-table-column label="操作" width="80">
                      <template #default="scope">
                        <el-button type="danger" link size="small" @click="removeKeyValue(from.podSelector.matchLabels, scope.row.key)">删除</el-button>
                      </template>
                    </el-table-column>
                  </el-table>
                  <div style="margin-top: 10px;">
                    <el-input v-model="from.podSelector.matchLabels.newKey" placeholder="键" style="width: 120px; margin-right: 5px;" size="small" />
                    <el-input v-model="from.podSelector.matchLabels.newValue" placeholder="值" style="width: 150px; margin-right: 5px;" size="small" />
                    <el-button type="primary" size="small" @click="addKeyValue(from.podSelector.matchLabels)">添加</el-button>
                  </div>
                </div>
                <div v-if="!from.podSelector.isEmpty" style="margin-top: 10px;">
                  <el-button type="primary" size="small" @click="toggleSelectorType(from, 'podSelector')">
                    {{ from.podSelector.selectorType === 'matchLabels' ? '切换到 MatchExpressions' : '切换到 MatchLabels' }}
                  </el-button>
                  <div v-if="from.podSelector.selectorType === 'matchExpressions'" style="margin-top: 10px;">
                    <div v-for="(expr, index) in from.podSelector.matchExpressions" :key="'pod-from-expr-' + index">
                      <el-input v-model="expr.key" placeholder="Key" style="width: 150px; margin-right: 5px;" />
                      <el-select v-model="expr.operator" placeholder="Operator" style="width: 120px; margin-right: 5px;">
                        <el-option label="In" value="In" />
                        <el-option label="NotIn" value="NotIn" />
                        <el-option label="Exists" value="Exists" />
                        <el-option label="DoesNotExist" value="DoesNotExist" />
                      </el-select>
                      <el-input
                          v-if="expr.operator === 'In' || expr.operator === 'NotIn'"
                          v-model="expr.values"
                          placeholder="Values (逗号分隔)"
                          style="width: 200px; margin-right: 5px;"
                      />
                      <el-button
                          type="danger" link icon="delete" size="small"
                          @click="from.podSelector.matchExpressions.splice(index, 1)"
                      >
                        删除
                      </el-button>
                    </div>
                    <el-button
                        size="small" type="primary" link icon="plus"
                        @click="from.podSelector.matchExpressions.push({ key: '', operator: '', values: '' })"
                    >
                      新增表达式
                    </el-button>
                  </div>
                </div>
              </template>

              <el-button
                  type="danger" link icon="delete" size="small"
                  @click="confirmDelete(() => rule.from.splice(fIndex, 1))"
                  :disabled="readOnly"
              >
                删除来源
              </el-button>
            </div>
            <el-button
                size="small" type="primary" link icon="plus"
                @click="addFromRule(rule)"
                :disabled="readOnly"
            >
              新增来源
            </el-button>

            <!-- Ports -->
            <div v-for="(port, pIndex) in rule.ports" :key="'port-' + pIndex" style="margin-top: 5px;">
              <el-select
                  v-model="port.protocol"
                  placeholder="协议"
                  style="width: 120px; margin-right: 5px;"
                  :disabled="readOnly"
              >
                <el-option label="TCP" value="TCP" />
                <el-option label="UDP" value="UDP" />
                <el-option label="SCTP" value="SCTP" />
              </el-select>
              <el-input
                  v-model="port.port"
                  placeholder="端口"
                  style="width: 120px; margin-right: 5px;"
                  :disabled="readOnly"
              />
              <el-input
                  v-model="port.endPort"
                  placeholder="结束端口 (可选)"
                  style="width: 120px; margin-right: 5px;"
                  :disabled="readOnly"
              />
              <el-button
                  type="danger" link icon="delete" size="small"
                  @click="confirmDelete(() => rule.ports.splice(pIndex, 1))"
                  :disabled="readOnly"
              >
                删除端口
              </el-button>
            </div>
            <el-button
                size="small" type="primary" link icon="plus"
                @click="rule.ports.push({ protocol: 'TCP', port: '', endPort: '' })"
                :disabled="readOnly"
            >
              新增端口
            </el-button>

            <el-button
                type="danger" link icon="delete" size="small"
                style="margin-top: 10px;"
                @click="confirmDelete(() => formData.spec.ingress.splice(rIndex, 1))"
                :disabled="readOnly"
            >
              删除该规则
            </el-button>
          </el-card>
        </div>

        <!-- Egress -->
        <el-divider content-position="left">Egress 规则</el-divider>
        <el-button
            size="small"
            type="primary"
            @click="addEgressRule"
            :disabled="readOnly"
        >
          新增 Egress
        </el-button>
        <div v-for="(rule, rIndex) in formData.spec.egress" :key="'egress-' + rIndex" class="rule-block">
          <el-card shadow="hover" style="margin-top: 10px;">
            <div v-for="(to, tIndex) in rule.to" :key="'to-' + tIndex" style="margin-bottom: 8px;">
              <el-select v-model="to.type" placeholder="选择类型" style="width: 150px" :disabled="readOnly">
                <el-option label="ipBlock" value="ipBlock" />
                <el-option label="namespaceSelector" value="namespaceSelector" />
                <el-option label="podSelector" value="podSelector" />
              </el-select>

              <template v-if="to.type === 'ipBlock' && to.ipBlock">
                <el-input
                    v-model="to.ipBlock.cidr"
                    placeholder="CIDR (如 10.0.0.0/24)"
                    style="width: 200px; margin-left: 10px;"
                    :disabled="readOnly"
                />
                <el-input
                    v-model="to.ipBlock.except"
                    placeholder="Except (如 10.0.0.1/32, 用逗号分隔)"
                    style="width: 200px; margin-left: 10px;"
                    :disabled="readOnly"
                />
              </template>

              <template v-else-if="to.type === 'namespaceSelector' && to.namespaceSelector">
                <el-checkbox v-model="to.namespaceSelector.isEmpty" label="匹配所有 Namespace" :disabled="readOnly" />
                <div v-if="!to.namespaceSelector.isEmpty">
                  <el-table :data="getObjectKeys(to.namespaceSelector.matchLabels).map(key => ({key, value: to.namespaceSelector.matchLabels[key]}))" style="width: 100%" size="small">
                    <el-table-column prop="key" label="键" width="120">
                      <template #default="scope">
                        <el-input v-model="scope.row.key" @blur="updateKeyValue(to.namespaceSelector.matchLabels, scope.$index, scope.row.key, scope.row.value)" size="small" />
                      </template>
                    </el-table-column>
                    <el-table-column prop="value" label="值">
                      <template #default="scope">
                        <el-input v-model="scope.row.value" @blur="updateKeyValue(to.namespaceSelector.matchLabels, scope.$index, scope.row.key, scope.row.value)" size="small" />
                      </template>
                    </el-table-column>
                    <el-table-column label="操作" width="80">
                      <template #default="scope">
                        <el-button type="danger" link size="small" @click="removeKeyValue(to.namespaceSelector.matchLabels, scope.row.key)">删除</el-button>
                      </template>
                    </el-table-column>
                  </el-table>
                  <div style="margin-top: 10px;">
                    <el-input v-model="to.namespaceSelector.matchLabels.newKey" placeholder="键" style="width: 120px; margin-right: 5px;" size="small" />
                    <el-input v-model="to.namespaceSelector.matchLabels.newValue" placeholder="值" style="width: 150px; margin-right: 5px;" size="small" />
                    <el-button type="primary" size="small" @click="addKeyValue(to.namespaceSelector.matchLabels)">添加</el-button>
                  </div>
                </div>
                <div v-if="!to.namespaceSelector.isEmpty" style="margin-top: 10px;">
                  <el-button type="primary" size="small" @click="toggleSelectorType(to, 'namespaceSelector')">
                    {{ to.namespaceSelector.selectorType === 'matchLabels' ? '切换到 MatchExpressions' : '切换到 MatchLabels' }}
                  </el-button>
                  <div v-if="to.namespaceSelector.selectorType === 'matchExpressions'" style="margin-top: 10px;">
                    <div v-for="(expr, index) in to.namespaceSelector.matchExpressions" :key="'ns-egress-expr-' + index">
                      <el-input v-model="expr.key" placeholder="Key" style="width: 150px; margin-right: 5px;" />
                      <el-select v-model="expr.operator" placeholder="Operator" style="width: 120px; margin-right: 5px;">
                        <el-option label="In" value="In" />
                        <el-option label="NotIn" value="NotIn" />
                        <el-option label="Exists" value="Exists" />
                        <el-option label="DoesNotExist" value="DoesNotExist" />
                      </el-select>
                      <el-input
                          v-if="expr.operator === 'In' || expr.operator === 'NotIn'"
                          v-model="expr.values"
                          placeholder="Values (逗号分隔)"
                          style="width: 200px; margin-right: 5px;"
                      />
                      <el-button
                          type="danger" link icon="delete" size="small"
                          @click="to.namespaceSelector.matchExpressions.splice(index, 1)"
                      >
                        删除
                      </el-button>
                    </div>
                    <el-button
                        size="small" type="primary" link icon="plus"
                        @click="to.namespaceSelector.matchExpressions.push({ key: '', operator: '', values: '' })"
                    >
                      新增表达式
                    </el-button>
                  </div>
                </div>
              </template>

              <template v-else-if="to.type === 'podSelector' && to.podSelector">
                <el-checkbox v-model="to.podSelector.isEmpty" label="匹配所有 Pod" :disabled="readOnly" />
                <div v-if="!to.podSelector.isEmpty">
                  <el-table :data="getObjectKeys(to.podSelector.matchLabels).map(key => ({key, value: to.podSelector.matchLabels[key]}))" style="width: 100%" size="small">
                    <el-table-column prop="key" label="键" width="120">
                      <template #default="scope">
                        <el-input v-model="scope.row.key" @blur="updateKeyValue(to.podSelector.matchLabels, scope.$index, scope.row.key, scope.row.value)" size="small" />
                      </template>
                    </el-table-column>
                    <el-table-column prop="value" label="值">
                      <template #default="scope">
                        <el-input v-model="scope.row.value" @blur="updateKeyValue(to.podSelector.matchLabels, scope.$index, scope.row.key, scope.row.value)" size="small" />
                      </template>
                    </el-table-column>
                    <el-table-column label="操作" width="80">
                      <template #default="scope">
                        <el-button type="danger" link size="small" @click="removeKeyValue(to.podSelector.matchLabels, scope.row.key)">删除</el-button>
                      </template>
                    </el-table-column>
                  </el-table>
                  <div style="margin-top: 10px;">
                    <el-input v-model="to.podSelector.matchLabels.newKey" placeholder="键" style="width: 120px; margin-right: 5px;" size="small" />
                    <el-input v-model="to.podSelector.matchLabels.newValue" placeholder="值" style="width: 150px; margin-right: 5px;" size="small" />
                    <el-button type="primary" size="small" @click="addKeyValue(to.podSelector.matchLabels)">添加</el-button>
                  </div>
                </div>
                <div v-if="!to.podSelector.isEmpty" style="margin-top: 10px;">
                  <el-button type="primary" size="small" @click="toggleSelectorType(to, 'podSelector')">
                    {{ to.podSelector.selectorType === 'matchLabels' ? '切换到 MatchExpressions' : '切换到 MatchLabels' }}
                  </el-button>
                  <div v-if="to.podSelector.selectorType === 'matchExpressions'" style="margin-top: 10px;">
                    <div v-for="(expr, index) in to.podSelector.matchExpressions" :key="'pod-to-expr-' + index">
                      <el-input v-model="expr.key" placeholder="Key" style="width: 150px; margin-right: 5px;" />
                      <el-select v-model="expr.operator" placeholder="Operator" style="width: 120px; margin-right: 5px;">
                        <el-option label="In" value="In" />
                        <el-option label="NotIn" value="NotIn" />
                        <el-option label="Exists" value="Exists" />
                        <el-option label="DoesNotExist" value="DoesNotExist" />
                      </el-select>
                      <el-input
                          v-if="expr.operator === 'In' || expr.operator === 'NotIn'"
                          v-model="expr.values"
                          placeholder="Values (逗号分隔)"
                          style="width: 200px; margin-right: 5px;"
                      />
                      <el-button
                          type="danger" link icon="delete" size="small"
                          @click="to.podSelector.matchExpressions.splice(index, 1)"
                      >
                        删除
                      </el-button>
                    </div>
                    <el-button
                        size="small" type="primary" link icon="plus"
                        @click="to.podSelector.matchExpressions.push({ key: '', operator: '', values: '' })"
                    >
                      新增表达式
                    </el-button>
                  </div>
                </div>
              </template>

              <el-button
                  type="danger" link icon="delete" size="small"
                  @click="confirmDelete(() => rule.to.splice(tIndex, 1))"
                  :disabled="readOnly"
              >
                删除目标
              </el-button>
            </div>
            <el-button
                size="small" type="primary" link icon="plus"
                @click="addToRule(rule)"
                :disabled="readOnly"
            >
              新增目标
            </el-button>

            <!-- Ports -->
            <div v-for="(port, pIndex) in rule.ports" :key="'egress-port-' + pIndex" style="margin-top: 5px;">
              <el-select
                  v-model="port.protocol"
                  placeholder="协议"
                  style="width: 120px; margin-right: 5px;"
                  :disabled="readOnly"
              >
                <el-option label="TCP" value="TCP" />
                <el-option label="UDP" value="UDP" />
                <el-option label="SCTP" value="SCTP" />
              </el-select>
              <el-input
                  v-model="port.port"
                  placeholder="端口"
                  style="width: 120px; margin-right: 5px;"
                  :disabled="readOnly"
              />
              <el-input
                  v-model="port.endPort"
                  placeholder="结束端口 (可选)"
                  style="width: 120px; margin-right: 5px;"
                  :disabled="readOnly"
              />
              <el-button
                  type="danger" link icon="delete" size="small"
                  @click="confirmDelete(() => rule.ports.splice(pIndex, 1))"
                  :disabled="readOnly"
              >
                删除端口
              </el-button>
            </div>
            <el-button
                size="small" type="primary" link icon="plus"
                @click="rule.ports.push({ protocol: 'TCP', port: '', endPort: '' })"
                :disabled="readOnly"
            >
              新增端口
            </el-button>

            <el-button
                type="danger" link icon="delete" size="small"
                @click="confirmDelete(() => formData.spec.egress.splice(rIndex, 1))"
                :disabled="readOnly"
            >
              删除该规则
            </el-button>
          </el-card>
        </div>

        <div class="grid-content bg-purple-light">
          <div style="float: right;margin-top: 10px">
            <el-button size="small" @click="closeDialog">取 消</el-button>
            <el-button size="small" type="primary" @click="enterDialog">确 定</el-button>
          </div>
        </div>
      </el-form>
    </el-card>
  </div>
</template>

<script>
export default {
  name: 'FormBlock',
}
</script>

<script setup>
import { ref, watch } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import FormItemBlock from '@/plugin/kubernetes/components/form-item/index.vue'
import NsSelect from '@/plugin/kubernetes/components/namespace/select-ns.vue'
// 移除 KVTableBlock 导入，改为内联实现

const emit = defineEmits(['close', 'enter'])
const props = defineProps({
  form: {
    default: function () {
      return {
        apiVersion: 'networking.k8s.io/v1',
        kind: 'NetworkPolicy',
        metadata: {
          name: '',
          namespace: '',
          labels: {},
          annotations: {},
        },
        spec: {
          podSelector: { isEmpty: false, matchLabels: {}, matchExpressions: [], selectorType: 'matchLabels' },
          policyTypes: [],
          ingress: [],
          egress: [],
        },
      }
    },
    type: Object,
  },
  cluster: {
    default: function () {
      return 0
    },
    type: Number,
  },
  readonly: {
    default: function () {
      return false
    },
    type: Boolean,
  },
})

// 数据初始化
const formData = ref({})
const cluster_id = ref({})
const readOnly = ref(false)
const FormBlock = ref(null)

const rules = ref({
  'metadata.name': [{ required: true, message: '请输入名称', trigger: 'blur' }],
  'metadata.namespace': [{ required: true, message: '请选择命名空间', trigger: 'blur' }],
  'spec.policyTypes': [
    {
      validator: (rule, value, callback) => {
        if (!value || value.length === 0) {
          callback(new Error('请至少选择一种策略类型 (Ingress 或 Egress)'))
        } else {
          callback()
        }
      },
      trigger: 'change',
    },
  ],
})

const initializeFormData = () => {
  const defaultForm = {
    apiVersion: 'networking.k8s.io/v1',
    kind: 'NetworkPolicy',
    metadata: {
      name: '',
      namespace: '',
      labels: { newKey: '', newValue: '' },
      annotations: { newKey: '', newValue: '' },
    },
    spec: {
      podSelector: { isEmpty: false, matchLabels: { newKey: '', newValue: '' }, matchExpressions: [], selectorType: 'matchLabels' },
      policyTypes: [],
      ingress: [],
      egress: [],
    },
  }
  formData.value = { ...defaultForm, ...JSON.parse(JSON.stringify(props.form)) }

  // 确保所有需要的对象和属性都存在
  if (!formData.value.metadata.labels) {
    formData.value.metadata.labels = {}
  }
  if (!formData.value.metadata.annotations) {
    formData.value.metadata.annotations = {}
  }

  // 确保所有需要的 newKey 和 newValue 属性都存在
  if (!formData.value.metadata.labels.hasOwnProperty('newKey')) {
    formData.value.metadata.labels.newKey = ''
  }
  if (!formData.value.metadata.labels.hasOwnProperty('newValue')) {
    formData.value.metadata.labels.newValue = ''
  }
  if (!formData.value.metadata.annotations.hasOwnProperty('newKey')) {
    formData.value.metadata.annotations.newKey = ''
  }
  if (!formData.value.metadata.annotations.hasOwnProperty('newValue')) {
    formData.value.metadata.annotations.newValue = ''
  }

  // Handle podSelector
  if (!formData.value.spec.podSelector || Object.keys(formData.value.spec.podSelector).length === 0) {
    formData.value.spec.podSelector = { isEmpty: true, matchLabels: {}, matchExpressions: [], selectorType: 'matchLabels' }
  } else {
    // 检查是否有 matchLabels 或 matchExpressions（排除 newKey 和 newValue 字段）
    const hasMatchLabels = formData.value.spec.podSelector.matchLabels &&
      Object.keys(formData.value.spec.podSelector.matchLabels).filter(key => key !== 'newKey' && key !== 'newValue').length > 0;
    const hasMatchExpressions = formData.value.spec.podSelector.matchExpressions && formData.value.spec.podSelector.matchExpressions.length > 0;

    // 保持用户的isEmpty选择，如果未定义则根据内容判断
    if (formData.value.spec.podSelector.isEmpty === undefined) {
      formData.value.spec.podSelector.isEmpty = !(hasMatchLabels || hasMatchExpressions);
    }

    formData.value.spec.podSelector.selectorType = hasMatchExpressions ? 'matchExpressions' : 'matchLabels';
    formData.value.spec.podSelector.matchLabels = formData.value.spec.podSelector.matchLabels || {};
    // 确保 podSelector.matchLabels 有 newKey 和 newValue 属性
    if (!formData.value.spec.podSelector.matchLabels.hasOwnProperty('newKey')) {
      formData.value.spec.podSelector.matchLabels.newKey = ''
    }
    if (!formData.value.spec.podSelector.matchLabels.hasOwnProperty('newValue')) {
      formData.value.spec.podSelector.matchLabels.newValue = ''
    }
    formData.value.spec.podSelector.matchExpressions = Array.isArray(formData.value.spec.podSelector.matchExpressions)
        ? formData.value.spec.podSelector.matchExpressions
        : [];
  }

  // Normalize ingress and egress rules
  formData.value.spec.ingress = (formData.value.spec.ingress || []).map(rule => ({
    from: (rule.from || []).map(from => normalizeSelector(from)),
    ports: rule.ports || [],
  }))
  formData.value.spec.egress = (formData.value.spec.egress || []).map(rule => ({
    to: (rule.to || []).map(to => normalizeSelector(to)),
    ports: rule.ports || [],
  }))
  cluster_id.value = props.cluster
  readOnly.value = props.readonly

}

// 获取默认的matchLabels，从NetworkPolicyObj中获取默认值
const getDefaultMatchLabels = (selectorType, namespace = formData.value?.metadata?.namespace) => {
  // 根据用户的需求，我们不再自动设置默认值
  // 返回空对象，保留用户在表单中设置的实际值
  return {};
};

// Normalize selector objects to ensure all properties exist
const normalizeSelector = (selector) => {
  let type = selector.type || 'podSelector'
  if (selector.namespaceSelector && Object.keys(selector.namespaceSelector).length > 0) {
    type = 'namespaceSelector'
  } else if (selector.ipBlock && Object.keys(selector.ipBlock).length > 0) {
    type = 'ipBlock'
  } else if (selector.podSelector && Object.keys(selector.podSelector).length > 0) {
    type = 'podSelector'
  }

  const defaultSelector = {
    type,
    ipBlock: selector.ipBlock || { cidr: '', except: '' },
    namespaceSelector: {
      // 保持用户的isEmpty选择，如果未定义则根据内容判断
      isEmpty: selector.namespaceSelector?.isEmpty !== undefined ? selector.namespaceSelector.isEmpty : (
        selector.namespaceSelector === undefined || (
          // 检查是否有真实的 matchLabels（排除 newKey 和 newValue）
          (!selector.namespaceSelector.matchLabels ||
           Object.keys(selector.namespaceSelector.matchLabels).filter(key => key !== 'newKey' && key !== 'newValue').length === 0) &&
          // 检查是否有 matchExpressions
          (!selector.namespaceSelector.matchExpressions || selector.namespaceSelector.matchExpressions.length === 0)
        )
      ),
      matchLabels: { ...(selector.namespaceSelector?.matchLabels || getDefaultMatchLabels('namespaceSelector')), newKey: '', newValue: '' },
      matchExpressions: Array.isArray(selector.namespaceSelector?.matchExpressions) ? selector.namespaceSelector.matchExpressions : [],
      selectorType: selector.namespaceSelector?.matchExpressions?.length > 0 ? 'matchExpressions' : 'matchLabels',
    },
    podSelector: {
      // 保持用户的isEmpty选择，如果未定义则根据内容判断
      isEmpty: selector.podSelector?.isEmpty !== undefined ? selector.podSelector.isEmpty : (
        selector.podSelector === undefined || (
          // 检查是否有真实的 matchLabels（排除 newKey 和 newValue）
          (!selector.podSelector.matchLabels ||
           Object.keys(selector.podSelector.matchLabels).filter(key => key !== 'newKey' && key !== 'newValue').length === 0) &&
          // 检查是否有 matchExpressions
          (!selector.podSelector.matchExpressions || selector.podSelector.matchExpressions.length === 0)
        )
      ),
      matchLabels: { ...(selector.podSelector?.matchLabels || getDefaultMatchLabels('podSelector')), newKey: '', newValue: '' },
      matchExpressions: Array.isArray(selector.podSelector?.matchExpressions) ? selector.podSelector.matchExpressions : [],
      selectorType: selector.podSelector?.matchExpressions?.length > 0 ? 'matchExpressions' : 'matchLabels',
    },
  }

  // 保留用户已设置的选择器值，不因为类型切换而清空
  // 只有当原始数据中完全没有该选择器时，才初始化为默认值
  if (!selector.namespaceSelector) {
    defaultSelector.namespaceSelector = {
      isEmpty: true,
      matchLabels: { ...getDefaultMatchLabels('namespaceSelector'), newKey: '', newValue: '' },
      matchExpressions: [],
      selectorType: 'matchLabels',
    }
  }
  if (!selector.podSelector) {
    defaultSelector.podSelector = {
      isEmpty: true,
      matchLabels: { ...getDefaultMatchLabels('podSelector'), newKey: '', newValue: '' },
      matchExpressions: [],
      selectorType: 'matchLabels',
    }
  }
  if (type !== 'ipBlock') {
    defaultSelector.ipBlock = { cidr: '', except: '' }
  }

  if (defaultSelector.ipBlock.except && Array.isArray(defaultSelector.ipBlock.except)) {
    defaultSelector.ipBlock.except = defaultSelector.ipBlock.except.join(',')
  } else if (defaultSelector.ipBlock.except === '') {
    defaultSelector.ipBlock.except = ''
  }

  // 正确合并选择器对象，确保不丢失重要属性
  const result = {
    type,
    ipBlock: defaultSelector.ipBlock,
    namespaceSelector: {
      ...defaultSelector.namespaceSelector,
      ...(selector.namespaceSelector || {})
    },
    podSelector: {
      ...defaultSelector.podSelector,
      ...(selector.podSelector || {})
    }
  }

  return result
}

// 创建默认的选择器配置
const createDefaultSelector = (type) => {
  const defaultConfig = { type, isEmpty: false, matchExpressions: [], selectorType: 'matchLabels' };

  // 根据类型设置不同的选择器
  if (type === 'podSelector') {
    defaultConfig.podSelector = {
      isEmpty: false,
      matchLabels: getDefaultMatchLabels('podSelector'),
      matchExpressions: [],
      selectorType: 'matchLabels'
    };
  } else if (type === 'namespaceSelector') {
    defaultConfig.namespaceSelector = {
      isEmpty: false,
      matchLabels: getDefaultMatchLabels('namespaceSelector'),
      matchExpressions: [],
      selectorType: 'matchLabels'
    };
  }
  return defaultConfig;
};

// Add new Ingress rule with a default from entry
const addIngressRule = () => {
  formData.value.spec.ingress.push({
    from: [normalizeSelector(createDefaultSelector('podSelector'))],
    ports: [],
  })
}

// Add new Egress rule with no default to entry
const addEgressRule = () => {
  formData.value.spec.egress.push({
    to: [],
    ports: [],
  })
}

// Add new from rule
const addFromRule = (rule) => {
  rule.from.push(normalizeSelector(createDefaultSelector('podSelector')))
}

// Add new to rule
const addToRule = (rule) => {
  rule.to.push(normalizeSelector(createDefaultSelector('podSelector')))
}

initializeFormData()

// 切换 PodSelector 的 matchLabels 和 matchExpressions
const togglePodSelectorType = () => {
  const podSelector = formData.value.spec.podSelector
  const previousMatchLabels = { ...podSelector.matchLabels }
  podSelector.selectorType = podSelector.selectorType === 'matchLabels' ? 'matchExpressions' : 'matchLabels'
  if (podSelector.selectorType === 'matchLabels') {
    podSelector.matchExpressions = []
    podSelector.matchLabels = Object.keys(previousMatchLabels).length > 0 ? previousMatchLabels : getDefaultMatchLabels('podSelector')
    // 计算 isEmpty 时排除 newKey 和 newValue 字段
    const realKeys = Object.keys(podSelector.matchLabels).filter(key => key !== 'newKey' && key !== 'newValue')
    podSelector.isEmpty = realKeys.length === 0
  } else {
    podSelector.matchLabels = {}
    podSelector.matchExpressions = Array.isArray(podSelector.matchExpressions) ? podSelector.matchExpressions : []
    if (podSelector.matchExpressions.length === 0) {
      podSelector.matchExpressions.push({ key: '', operator: '', values: '' })
    }
    podSelector.isEmpty = false
  }
}

// 切换 namespaceSelector 或 podSelector 的 matchLabels 和 matchExpressions
const toggleSelectorType = (item, selectorKey) => {
  const selector = item[selectorKey]
  const previousMatchLabels = { ...selector.matchLabels }
  selector.selectorType = selector.selectorType === 'matchLabels' ? 'matchExpressions' : 'matchLabels'
  if (selector.selectorType === 'matchLabels') {
    selector.matchExpressions = []
    selector.matchLabels = Object.keys(previousMatchLabels).length > 0 ? previousMatchLabels : getDefaultMatchLabels(selectorKey)
    // 计算 isEmpty 时排除 newKey 和 newValue 字段
    const realKeys = Object.keys(selector.matchLabels).filter(key => key !== 'newKey' && key !== 'newValue')
    selector.isEmpty = realKeys.length === 0
  } else {
    selector.matchLabels = {}
    selector.matchExpressions = Array.isArray(selector.matchExpressions) ? selector.matchExpressions : []
    if (selector.matchExpressions.length === 0) {
      selector.matchExpressions.push({ key: '', operator: '', values: '' })
    }
    selector.isEmpty = false
  }
}

// 删除确认
const confirmDelete = (callback) => {
  ElMessageBox.confirm('确定删除此项？', '警告', {
    confirmButtonText: '确定',
    cancelButtonText: '取消',
    type: 'warning',
  })
      .then(() => {
        callback()
        ElMessage.success('删除成功')
      })
      .catch(() => {
        ElMessage.info('已取消删除')
      })
}

// 键值对管理函数
const addKeyValue = (obj) => {
  if (!obj) return
  if (!obj.newKey) obj.newKey = ''
  if (!obj.newValue) obj.newValue = ''
  if (obj.newKey.trim() !== '') {
    obj[obj.newKey] = obj.newValue
    obj.newKey = ''
    obj.newValue = ''
  }
}

const removeKeyValue = (obj, key) => {
  if (obj && key) {
    delete obj[key]
  }
}

const getObjectKeys = (obj) => {
  if (!obj) return []
  return Object.keys(obj).filter(key => key !== 'newKey' && key !== 'newValue')
}

const updateKeyValue = (obj, index, newKey, newValue) => {
  if (!obj) return
  const keys = getObjectKeys(obj)
  const oldKey = keys[index]
  if (oldKey && oldKey !== newKey) {
    delete obj[oldKey]
  }
  if (newKey && newKey.trim() !== '') {
    obj[newKey] = newValue || ''
  }
}

// 关闭
const closeDialog = () => {
  FormBlock.value?.resetFields()
  emit('close')
}

// 提交
const labels = ref(null)
const annotations = ref(null)
const enterDialog = async () => {
  try {
    if (!FormBlock.value) {
      console.error('FormBlock reference is null')
      throw new Error('表单引用未初始化')
    }
    await FormBlock.value.validate((valid, fields) => {
      if (!valid) {
        console.error('Validation failed:', fields)
        throw new Error('表单验证失败')
      }
    })
    // Transform labels and annotations safely
    try {
      if (labels.value?.transformationformat) {
        await labels.value.transformationformat(formData.value.metadata, 'labels')
      }
      if (annotations.value?.transformationformat) {
        await annotations.value.transformationformat(formData.value.metadata, 'annotations')
      }
    } catch (transformError) {
      console.error('Transformation error:', transformError)
      throw new Error('标签或注解转换失败')
    }
    // Clean up formData for submission
    const cleanedFormData = JSON.parse(JSON.stringify(formData.value))

    // 递归清理 newKey 和 newValue 字段
    const cleanNewKeyValue = (obj) => {
      if (obj && typeof obj === 'object') {
        if (Array.isArray(obj)) {
          obj.forEach(item => cleanNewKeyValue(item))
        } else {
          // 删除 newKey 和 newValue 字段
          delete obj.newKey
          delete obj.newValue
          // 递归处理所有属性
          Object.values(obj).forEach(value => cleanNewKeyValue(value))
        }
      }
    }

    cleanNewKeyValue(cleanedFormData)
    // 根据用户的选择处理 podSelector
    if (cleanedFormData.spec.podSelector.isEmpty) {
      // 用户选择了"匹配所有 Pod"，设置为空对象
      cleanedFormData.spec.podSelector = {}
    } else if (cleanedFormData.spec.podSelector.selectorType === 'matchLabels') {
      // 用户设置了具体的 matchLabels
      cleanedFormData.spec.podSelector = {
        matchLabels: cleanedFormData.spec.podSelector.matchLabels || {}
      }
    } else {
      // 用户设置了 matchExpressions
      cleanedFormData.spec.podSelector.matchExpressions = Array.isArray(cleanedFormData.spec.podSelector.matchExpressions)
          ? cleanedFormData.spec.podSelector.matchExpressions.map((expr) => ({
            ...expr,
            values: expr.values ? expr.values.split(',').map((v) => v.trim()).filter(v => v) : [],
          }))
          : []
      if (cleanedFormData.spec.podSelector.matchExpressions.length === 0) {
        // 如果没有有效的 matchExpressions，设置为空对象
        cleanedFormData.spec.podSelector = {}
      } else {
        // 保留 matchExpressions 数据
        cleanedFormData.spec.podSelector = {
          matchExpressions: cleanedFormData.spec.podSelector.matchExpressions
        }
      }
    }
    // Clean up ingress and egress selectors
    ;['ingress', 'egress'].forEach((type) => {
      cleanedFormData.spec[type] = cleanedFormData.spec[type] || []
      cleanedFormData.spec[type].forEach((rule) => {
        const key = type === 'ingress' ? 'from' : 'to'
        rule[key] = rule[key] || []
        rule.ports = rule.ports || []
        rule[key].forEach((item) => {
          if (item.type === 'namespaceSelector' || item.type === 'podSelector') {
            if (!item[item.type]) {
              item[item.type] = {
                isEmpty: true,
                matchLabels: getDefaultMatchLabels(item.type),
                matchExpressions: [],
                selectorType: 'matchLabels'
              }
            }
            // 根据用户的选择处理选择器
            if (item[item.type].isEmpty) {
              // 用户选择了"匹配所有"，设置为空对象
              item[item.type] = {}
            } else if (item[item.type].selectorType === 'matchLabels') {
              // 用户设置了具体的 matchLabels，保留用户的实际设置
              const userMatchLabels = item[item.type].matchLabels
              item[item.type] = {
                matchLabels: userMatchLabels
              }
            } else {
              item[item.type].matchExpressions = Array.isArray(item[item.type].matchExpressions)
                  ? item[item.type].matchExpressions.map((expr) => ({
                    ...expr,
                    values: expr.values ? expr.values.split(',').map((v) => v.trim()).filter(v => v) : [],
                  }))
                  : []
              if (item[item.type].matchExpressions.length === 0) {
                item[item.type] = {}
              } else {
                // 保留matchExpressions数据，删除其他属性
                item[item.type] = {
                  matchExpressions: item[item.type].matchExpressions
                }
              }
            }
          } else if (item.type === 'ipBlock') {
            if (!item.ipBlock) {
              item.ipBlock = { cidr: '', except: '' }
            }
            item.ipBlock.except = item.ipBlock.except
                ? item.ipBlock.except.split(',').map((e) => e.trim()).filter(e => e)
                : []
            if (!item.ipBlock.cidr) {
              delete item.ipBlock
            }
          }
          // Remove unused selector types
          if (item.type !== 'ipBlock') delete item.ipBlock
          if (item.type !== 'namespaceSelector') delete item.namespaceSelector
          if (item.type !== 'podSelector') delete item.podSelector

          // 删除type字段，确保在删除其他选择器后执行，避免数据丢失
          delete item.type
        })
        if (Array.isArray(rule.ports)) {
          rule.ports = rule.ports.filter((port) => port.port || port.endPort)
          rule.ports.forEach((port) => {
            if (!port.endPort) delete port.endPort
            port.port = port.port ? parseInt(port.port, 10) : port.port
          })
        }
        if (rule.ports.length === 0) delete rule.ports
      })
      if (cleanedFormData.spec[type].length === 0) delete cleanedFormData.spec[type]
    })

    emit('enter', cleanedFormData)
    ElMessage.success('提交成功')
  } catch (error) {
    console.error('Enter dialog error:', error)
    ElMessage.error(error.message || '表单验证失败，请检查输入')
  }
}

// Watch for changes in from/to type to ensure proper initialization
watch(
    () => formData.value.spec.ingress,
    (newIngress) => {
      newIngress.forEach((rule) => {
        rule.from = rule.from || []
        rule.ports = rule.ports || []
        rule.from.forEach((from) => {
          if (!from.ipBlock) from.ipBlock = { cidr: '', except: '' }
          if (!from.namespaceSelector) {
            from.namespaceSelector = {
              isEmpty: true,
              matchLabels: { ...getDefaultMatchLabels('namespaceSelector'), newKey: '', newValue: '' },
              matchExpressions: [],
              selectorType: 'matchLabels',
            }
          } else {
            // 计算 isEmpty 时排除 newKey 和 newValue 字段
            const hasRealMatchLabels = from.namespaceSelector.matchLabels &&
              Object.keys(from.namespaceSelector.matchLabels).filter(key => key !== 'newKey' && key !== 'newValue').length > 0
            // 保持用户的isEmpty选择，如果未定义则根据内容判断
            if (from.namespaceSelector.isEmpty === undefined) {
              from.namespaceSelector.isEmpty = !(
                  hasRealMatchLabels ||
                  (from.namespaceSelector.matchExpressions && from.namespaceSelector.matchExpressions.length > 0)
              )
            }
            from.namespaceSelector.matchLabels = from.namespaceSelector.matchLabels || {}
            from.namespaceSelector.matchExpressions = Array.isArray(from.namespaceSelector.matchExpressions)
                ? from.namespaceSelector.matchExpressions
                : []
            from.namespaceSelector.selectorType = from.namespaceSelector.matchExpressions.length > 0 ? 'matchExpressions' : 'matchLabels'
          }
          if (!from.podSelector) {
            from.podSelector = {
              isEmpty: true,
              matchLabels: getDefaultMatchLabels('podSelector'),
              matchExpressions: [],
              selectorType: 'matchLabels',
            }
          } else {
            // 计算 isEmpty 时排除 newKey 和 newValue 字段
            const hasRealPodMatchLabels = from.podSelector.matchLabels &&
              Object.keys(from.podSelector.matchLabels).filter(key => key !== 'newKey' && key !== 'newValue').length > 0
            // 保持用户的isEmpty选择，如果未定义则根据内容判断
            if (from.podSelector.isEmpty === undefined) {
              from.podSelector.isEmpty = !(
                  hasRealPodMatchLabels ||
                  (from.podSelector.matchExpressions && from.podSelector.matchExpressions.length > 0)
              )
            }
            from.podSelector.matchLabels = from.podSelector.matchLabels || {}
            from.podSelector.matchExpressions = Array.isArray(from.podSelector.matchExpressions)
                ? from.podSelector.matchExpressions
                : []
            from.podSelector.selectorType = from.podSelector.matchExpressions.length > 0 ? 'matchExpressions' : 'matchLabels'
          }
        })
      })
    },
    { deep: true }
)

watch(
    () => formData.value.spec.egress,
    (newEgress) => {
      newEgress.forEach((rule) => {
        rule.to = rule.to || []
        rule.ports = rule.ports || []
        rule.to.forEach((to) => {
          if (!to.ipBlock) to.ipBlock = { cidr: '', except: '' }
          if (!to.namespaceSelector) {
            to.namespaceSelector = {
              isEmpty: true,
              matchLabels: getDefaultMatchLabels('namespaceSelector'),
              matchExpressions: [],
              selectorType: 'matchLabels',
            }
          } else {
            // 计算 isEmpty 时排除 newKey 和 newValue 字段
            const hasRealMatchLabels = to.namespaceSelector.matchLabels &&
              Object.keys(to.namespaceSelector.matchLabels).filter(key => key !== 'newKey' && key !== 'newValue').length > 0
            // 保持用户的isEmpty选择，如果未定义则根据内容判断
            if (to.namespaceSelector.isEmpty === undefined) {
              to.namespaceSelector.isEmpty = !(
                  hasRealMatchLabels ||
                  (to.namespaceSelector.matchExpressions && to.namespaceSelector.matchExpressions.length > 0)
              )
            }
            to.namespaceSelector.matchLabels = to.namespaceSelector.matchLabels || {}
            to.namespaceSelector.matchExpressions = Array.isArray(to.namespaceSelector.matchExpressions)
                ? to.namespaceSelector.matchExpressions
                : []
            to.namespaceSelector.selectorType = to.namespaceSelector.matchExpressions.length > 0 ? 'matchExpressions' : 'matchLabels'
          }
          if (!to.podSelector) {
            to.podSelector = {
              isEmpty: true,
              matchLabels: getDefaultMatchLabels('podSelector'),
              matchExpressions: [],
              selectorType: 'matchLabels',
            }
          } else {
            // 计算 isEmpty 时排除 newKey 和 newValue 字段
            const hasRealPodMatchLabels = to.podSelector.matchLabels &&
              Object.keys(to.podSelector.matchLabels).filter(key => key !== 'newKey' && key !== 'newValue').length > 0
            // 保持用户的isEmpty选择，如果未定义则根据内容判断
            if (to.podSelector.isEmpty === undefined) {
              to.podSelector.isEmpty = !(
                  hasRealPodMatchLabels ||
                  (to.podSelector.matchExpressions && to.podSelector.matchExpressions.length > 0)
              )
            }
            to.podSelector.matchLabels = to.podSelector.matchLabels || {}
            to.podSelector.matchExpressions = Array.isArray(to.podSelector.matchExpressions)
                ? to.podSelector.matchExpressions
                : []
            to.podSelector.selectorType = to.podSelector.matchExpressions.length > 0 ? 'matchExpressions' : 'matchLabels'
          }
        })
      })
    },
    {deep: true}
)

// Watch for isEmpty changes in podSelector
watch(
    () => formData.value.spec.podSelector.isEmpty,
    (newIsEmpty) => {
      if (newIsEmpty) {
        // 当选择"匹配所有Pod"时，清空matchLabels和matchExpressions
        formData.value.spec.podSelector.matchLabels = { newKey: '', newValue: '' }
        formData.value.spec.podSelector.matchExpressions = []
      }
    }
)
</script>

<style scoped>
.rule-block {
  margin-top: 10px;
}

/* 优化输入框样式 */
.el-input {
  margin-right: 8px;
  margin-bottom: 8px;
}

.el-input .el-input__wrapper {
  border-radius: 6px;
  box-shadow: 0 1px 3px rgba(0, 0, 0, 0.1);
  transition: all 0.3s ease;
  border: 1px solid #dcdfe6;
}

.el-input .el-input__wrapper:hover {
  border-color: #c0c4cc;
  box-shadow: 0 2px 6px rgba(0, 0, 0, 0.15);
}

.el-input.is-focus .el-input__wrapper {
  border-color: #409eff;
  box-shadow: 0 0 0 2px rgba(64, 158, 255, 0.2);
}

.el-input .el-input__inner {
  padding: 8px 12px;
  font-size: 14px;
  line-height: 1.5;
}

/* 优化选择框样式 */
.el-select {
  margin-right: 8px;
  margin-bottom: 8px;
}

.el-select .el-input__wrapper {
  border-radius: 6px;
  box-shadow: 0 1px 3px rgba(0, 0, 0, 0.1);
  transition: all 0.3s ease;
  border: 1px solid #dcdfe6;
}

.el-select .el-input__wrapper:hover {
  border-color: #c0c4cc;
  box-shadow: 0 2px 6px rgba(0, 0, 0, 0.15);
}

.el-select.is-focus .el-input__wrapper {
  border-color: #409eff;
  box-shadow: 0 0 0 2px rgba(64, 158, 255, 0.2);
}

/* 表单项容器优化 */
.form-input-group {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 8px;
  margin-top: 10px;
}

.form-input-group .el-input,
.form-input-group .el-select {
  margin-right: 0;
  margin-bottom: 0;
}

/* 小尺寸输入框优化 */
.el-input--small .el-input__inner {
  padding: 6px 10px;
  font-size: 13px;
}

/* 深色模式适配 */
.dark .el-input .el-input__wrapper {
  background-color: #2d3748;
  border-color: #4a5568;
}

.dark .el-input .el-input__wrapper:hover {
  border-color: #718096;
}

.dark .el-input.is-focus .el-input__wrapper {
  border-color: #409eff;
}

.dark .el-select .el-input__wrapper {
  background-color: #2d3748;
  border-color: #4a5568;
}

.dark .el-select .el-input__wrapper:hover {
  border-color: #718096;
}

.dark .el-select.is-focus .el-input__wrapper {
  border-color: #409eff;
}
</style>
