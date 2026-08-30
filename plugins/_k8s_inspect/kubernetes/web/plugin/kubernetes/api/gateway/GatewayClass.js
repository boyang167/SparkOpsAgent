import service from '@/utils/request'

const BaseUrl = (cluster_id) => {
    return `/kubernetes/proxy/${cluster_id}/apis/gateway.networking.k8s.io/v1/gatewayclasses`
}

export const GatewayClassList = (cluster_id, page, pageSize) => {
    return service({
        url: `${BaseUrl(cluster_id)}?search=true&page=${page}&pageSize=${pageSize}`,
        method: 'get',
    })
}

export const GatewayClassGet = (cluster_id, name) => {
    return service({
        url: `${BaseUrl(cluster_id)}/${name}`,
        method: 'get',
    })
}

export const GatewayClassCreate = (cluster_id, data) => {
    return service({
        url: `${BaseUrl(cluster_id)}`,
        method: 'post',
        data
    })
}

export const GatewayClassUpdate = (cluster_id, name,data) => {
    return service({
        url: `${BaseUrl(cluster_id)}/${name}`,
        method: 'put',
        data
    })
}

export const GatewayClassPatch = (cluster_id, name, data) => {
    return service({
        url: `${BaseUrl(cluster_id)}/${name}`,
        method: 'patch',
        data
    })
}

export const GatewayClassDelete = (cluster_id, name) => {
    return service({
        url: `${BaseUrl(cluster_id)}/${name}`,
        method: 'delete',
    })
}
