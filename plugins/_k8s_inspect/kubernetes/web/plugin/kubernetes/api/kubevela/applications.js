import service from '@/utils/request'

const BaseVelauxUrl = (cluster_id) => {
  return `/kubernetes/velaux/${cluster_id}/api/v1/query`
}

const BaseApplicationsVelauxUrl = (cluster_id) => {
  return `/kubernetes/velaux/${cluster_id}/api/v1/applications`
}

const BaseNamespaceUrl = (cluster_id, namespace) => {
  return `/kubernetes/proxy/${cluster_id}/apis/core.oam.dev/v1beta1/namespaces/${namespace}/applications`
}

export const ApplicationsList = (cluster_id, namespace, labelSelector, page, pageSize) => {
  if (namespace && namespace !== 'All Namespaces') {
    return service({
      url: `${BaseNamespaceUrl(cluster_id, namespace)}?search=true&labelSelector=${labelSelector}&page=${page}&pageSize=${pageSize}`,
      method: 'get',
    })
  }

  return service({
    url: `${BaseVelauxUrl(cluster_id)}`,
    method: 'get',
  })
}

export const ApplicationsGet = (cluster_id, namespace, name) => {
  return service({
    url: `${BaseNamespaceUrl(cluster_id, namespace)}/${name}`,
    method: 'get',
  })
}

export const ApplicationsCreate = (cluster_id, namespace, data) => {
  return service({
    url: `${BaseNamespaceUrl(cluster_id, namespace)}`,
    method: 'post',
    data
  })
}

export const ApplicationsUpdate = (cluster_id, namespace, name, data) => {
  return service({
    url: `${BaseNamespaceUrl(cluster_id, namespace)}/${name}`,
    method: 'put',
    data
  })
}

export const ApplicationsDelete = (cluster_id, namespace, name) => {
  return service({
    url: `${BaseNamespaceUrl(cluster_id, namespace)}/${name}`,
    method: 'delete',
  })
}

export const ApplicationStatisticsGet = (cluster_id, name) => {
  return service({
    url: `${BaseApplicationsVelauxUrl(cluster_id)}/${name}/statistics`,
    method: 'get',
  })
}

export const ApplicationVelaqlList = (cluster_id, velaql) => {
  return service({
    url: `${BaseVelauxUrl(cluster_id)}?velaql=${velaql}`,
    method: 'get',
  })
}

export const ApplicationRevisionsList = (cluster_id, app_name, page, pagesize) => {
  return service({
    url: `${BaseApplicationsVelauxUrl(cluster_id)}/${app_name}/revisions?appName=${app_name}&envName=&status=&page=${page}&pageSize=${pagesize}`,
    method: 'get',
  })
}

export const ApplicationRevisionsGet = (cluster_id, app_name, version) => {
  return service({
    url: `${BaseApplicationsVelauxUrl(cluster_id)}/${app_name}/revisions/${version}`,
    method: 'get',
  })
}

export const ApplicationWorkflowsList = (cluster_id, app_name) => {
  return service({
    url: `${BaseApplicationsVelauxUrl(cluster_id)}/${app_name}/workflows`,
    method: 'get',
  })
}
export const ApplicationWorkflowsResume = (cluster_id, app_name, workflowName, workflowVersion) => {
  return service({
    url: `${BaseApplicationsVelauxUrl(cluster_id)}/${app_name}/workflows/${workflowName}/records/${workflowVersion}/resume`,
    method: 'get',
  })
}

export const ApplicationWorkflowsTerminate = (cluster_id, app_name, workflowName, workflowVersion) => {
  return service({
    url: `${BaseApplicationsVelauxUrl(cluster_id)}/${app_name}/workflows/${workflowName}/records/${workflowVersion}/terminate`,
    method: 'get',
  })
}
