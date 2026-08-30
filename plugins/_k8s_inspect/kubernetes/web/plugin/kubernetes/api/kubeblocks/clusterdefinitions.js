import service from '@/utils/request'

const BaseUrl = (cluster_id) => {
  return `/kubernetes/proxy/${cluster_id}/apis/apps.kubeblocks.io/v1alpha1/clusterdefinitions`
}

export const ClusterDefinitionsList = (cluster_id) => {
  return service({
    url: `${BaseUrl(cluster_id)}?search=true`,
    method: 'get',
  })
}

