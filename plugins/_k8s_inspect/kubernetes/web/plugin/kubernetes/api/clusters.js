import service from '@/utils/request'

export const getClustersList = (data) => {
  return service({
    url: '/kubernetes/clusters/list',
    method: 'post',
    data
  })
}

export const getClustersById = (data) => {
  return service({
    url: '/kubernetes/clusters/getById',
    method: 'post',
    data
  })
}

export const CreateCluster = (data) => {
  return service({
    url: '/kubernetes/clusters/create',
    method: 'post',
    data
  })
}

export const UpdateCluster = (data) => {
  return service({
    url: '/kubernetes/clusters/update',
    method: 'put',
    data
  })
}

export const DeleteCluster = (data) => {
  return service({
    url: '/kubernetes/clusters/delete',
    method: 'delete',
    data
  })
}

export const DeleteClusterByIds = (data) => {
  return service({
    url: '/kubernetes/clusters/deleteByIds',
    method: 'delete',
    data
  })
}

export const CreateCredential = (data) => {
  return service({
    url: '/kubernetes/clusters/createCredential',
    method: 'post',
    data
  })
}

export const getUserById = (data) => {
  return service({
    url: '/kubernetes/clusters/getUserById',
    method: 'post',
    data
  })
}

export const getClusterRoles = (data) => {
  return service({
    url: '/kubernetes/clusters/getClusterRoles',
    method: 'post',
    data
  })
}

export const getClusterApiGroups = (data) => {
  return service({
    url: '/kubernetes/clusters/getClusterApiGroups',
    method: 'post',
    data
  })
}

export const createClusterRole = (data) => {
  return service({
    url: '/kubernetes/clusters/createClusterRole',
    method: 'post',
    data
  })
}

export const updateClusterRole = (data) => {
  return service({
    url: '/kubernetes/clusters/updateClusterRole',
    method: 'put',
    data
  })
}

export const deleteClusterRole = (data) => {
  return service({
    url: '/kubernetes/clusters/deleteClusterRole',
    method: 'delete',
    data
  })
}

export const createClusterUser = (data) => {
  return service({
    url: '/kubernetes/clusters/createClusterUser',
    method: 'post',
    data
  })
}

export const updateClusterUser = (data) => {
  return service({
    url: '/kubernetes/clusters/updateClusterUser',
    method: 'put',
    data
  })
}

export const deleteClusterUser = (data) => {
  return service({
    url: '/kubernetes/clusters/deleteClusterUser',
    method: 'delete',
    data
  })
}