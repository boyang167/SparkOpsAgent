import service from '@/utils/request'

export const diagnosisInstall = (data) => {
    return service({
        url: '/kubernetes/diagnosis/install',
        method: 'post',
        donNotShowLoading: true,
        data
    })
}

export const diagnosisUninstall = (data) => {
    return service({
        url: '/kubernetes/diagnosis/uninstall',
        method: 'DELETE',
        donNotShowLoading: true,
        data
    })
}

export const diagnosisGetByPod = (data) => {
    return service({
        url: '/kubernetes/diagnosis/getByPod',
        method: 'post',
        donNotShowLoading: true,
        data
    })
}

export const diagnosisProxy = (data) => {
    return service({
        url: '/kubernetes/diagnosis/proxy',
        method: 'post',
        donNotShowLoading: true,
        data
    })
}

export const diagnosisJps = (data) => {
    return service({
        url: '/kubernetes/diagnosis/jps',
        method: 'post',
        donNotShowLoading: false,
        data
    })
}
