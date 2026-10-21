import service from '@/utils/request'

const BaseUrl = (cluster_id) => {
  return `/kubernetes/proxy/${cluster_id}/apis/apps.kubeblocks.io/v1alpha1/clusterversions`
}

export const ClusterVersionsList = (cluster_id) => {
  return service({
    url: `${BaseUrl(cluster_id)}?search=true`,
    method: 'get',
  })
}

export const ClusterVersionsGet = (cluster_id, name) => {
  return service({
    url: `${BaseUrl(cluster_id)}/${name}`,
    method: 'get',
  })
}
