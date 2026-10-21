package service

import (
	"fmt"
	"github.com/flipped-aurora/gin-vue-admin/server/global"
	"github.com/flipped-aurora/gin-vue-admin/server/model/common/request"
	"github.com/flipped-aurora/gin-vue-admin/server/plugin/kubernetes/model"
	kubernetesReq "github.com/flipped-aurora/gin-vue-admin/server/plugin/kubernetes/model/kubernetes/request"
	"github.com/flipped-aurora/gin-vue-admin/server/plugin/kubernetes/utils"
)

type RecordsService struct{}

//@function: ListRecord
//@description: 获取终端录制记录
//@param: req kubernetesReq.SearchRecordParams
//@return: (list interface{}, total int64, err error)

func (p *RecordsService) ListRecord(info kubernetesReq.SearchRecordParams) (list interface{}, total int64, err error) {
	limit := info.PageSize
	offset := info.PageSize * (info.Page - 1)
	db := global.GVA_DB.Model(&model.PodRecord{}).Select("id, uuid, cluster, namespace, pod_name, container_name, username, nick_name, created_at")
	var recordsList []model.PodRecord

	if info.PodName != "" {
		db = db.Where("pod_name LIKE ?", "%"+info.PodName+"%")
	}

	err = db.Count(&total).Error

	if err != nil {
		return recordsList, total, err
	} else {
		db = db.Limit(limit).Offset(offset)
		if info.OrderKey != "" {
			var OrderStr string
			// 设置有效排序key 防止sql注入
			orderMap := make(map[string]bool, 5)
			orderMap["id"] = true
			orderMap["cluster"] = true
			orderMap["namespace"] = true
			orderMap["pod_name"] = true
			orderMap["container_name"] = true
			orderMap["username"] = true
			orderMap["nick_name"] = true
			if orderMap[info.OrderKey] {
				if info.Desc {
					OrderStr = info.OrderKey + " desc"
				} else {
					OrderStr = info.OrderKey
				}
			} else { // didn't matched any order key in `orderMap`
				err = fmt.Errorf("非法的排序字段: %v", info.OrderKey)
				return recordsList, total, err
			}

			err = db.Order(OrderStr).Find(&recordsList).Error
		} else {
			err = db.Order("id desc").Find(&recordsList).Error
		}
	}

	return recordsList, total, err
}

//@function: GetRecordById
//@description: 根据id获取记录
//@param: id float64
//@return: (record model.PodRecord, err error)

func (p *RecordsService) GetRecordById(id int) (recordByte []byte, err error) {
	var record model.PodRecord
	err = global.GVA_DB.Where("id = ?", id).First(&record).Error

	return utils.DoZlibUnCompress(record.Records), err
}

//@author: Eagle
//@function: DeleteRecord
//@description: 删除
//@param: req request.GetById
//@return: err error

func (p *RecordsService) DeleteRecord(id int) (err error) {
	return global.GVA_DB.Where("id = ?", id).Delete(&model.PodRecord{}).Error
}

//@author: Eagle
//@function: DeleteRecordByIds
//@description: 删除
//@param: ids request.IdsReq
//@return: err error

func (p *RecordsService) DeleteRecordByIds(ids request.IdsReq) (err error) {
	return global.GVA_DB.Delete(&[]model.PodRecord{}, "id in ?", ids.Ids).Error
}
