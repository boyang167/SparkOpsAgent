import service from '@/utils/request'

const BaseUrl = (cluster_id) => {
    return `/kubernetes/proxy/${cluster_id}/apis/gateway.networking.k8s.io/v1/grpcroutes`
}

const BaseNamespaceUrl = (cluster_id, namespace) => {
    return `/kubernetes/proxy/${cluster_id}/apis/gateway.networking.k8s.io/v1/namespaces/${namespace}/grpcroutes`
}

export const GRPCRouteList = (cluster_id, page, pageSize, namespace, keywords) => {
    if (namespace && namespace !== 'All Namespaces') {
        return service({
            url: `${BaseNamespaceUrl(cluster_id, namespace)}?search=true&keywords=${keywords}&page=${page}&pageSize=${pageSize}`,
            method: 'get',
        })
    }
    return service({
        url: `${BaseUrl(cluster_id)}?search=true&keywords=${keywords}&page=${page}&pageSize=${pageSize}`,
        method: 'get',
    })
}

export const GRPCRouteGet = (cluster_id, namespace, name) => {
    return service({
        url: `${BaseNamespaceUrl(cluster_id, namespace)}/${name}`,
        method: 'get',
    })
}

export const GRPCRouteCreate = (cluster_id, namespace, data) => {
    return service({
        url: `${BaseNamespaceUrl(cluster_id, namespace)}`,
        method: 'post',
        data
    })
}

export const GRPCRouteUpdate = (cluster_id, namespace, name,data) => {
    return service({
        url: `${BaseNamespaceUrl(cluster_id, namespace)}/${name}?namespace=${namespace}`,
        method: 'put',
        data
    })
}

export const GRPCRoutePatch = (cluster_id, namespace, name, data) => {
    return service({
        url: `${BaseNamespaceUrl(cluster_id, namespace)}/${name}?namespace=${namespace}`,
        method: 'patch',
        data
    })
}

export const GRPCRouteDelete = (cluster_id, namespace, name) => {
    return service({
        url: `${BaseNamespaceUrl(cluster_id, namespace)}/${name}`,
        method: 'delete',
    })
}
