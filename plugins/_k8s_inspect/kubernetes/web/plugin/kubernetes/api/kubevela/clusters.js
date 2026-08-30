import service from '@/utils/request'

const BaseUrl = (cluster_id) => {
  return `/kubernetes/velaux/${cluster_id}/api/v1/clusters`
}

const BaseVelauxUrl = (cluster_id) => {
  return `/kubernetes/velaux/${cluster_id}/api/v1/clusters`
}

export const ClustersList = (cluster_id, page, pageSize, query) => {
  return service({
    url: `${BaseVelauxUrl(cluster_id)}?query=${query}&page=${page}&pageSize=${pageSize}`,
    method: 'get',
  })
}

export const ClustersJoin = (cluster_id, data) => {
  return service({
    url: `${BaseUrl(cluster_id)}`,
    method: 'post',
    data
  })
}

export const ClustersDetail = (cluster_id, name) => {
  return service({
    url: `${BaseUrl(cluster_id)}/${name}`,
    method: 'get',
  })
}

export const ClustersUpdate = (cluster_id, name, data) => {
  return service({
    url: `${BaseUrl(cluster_id)}/${name}`,
    method: 'put',
    data
  })
}

export const ClustersDelete = (cluster_id, name) => {
  return service({
    url: `${BaseUrl(cluster_id)}/${name}`,
    method: 'delete',
  })
}
