package model

/**
 * @Author: wang hao
 * @Date: 2021/12/23 17:00
 * @Description:机器管理树形结构体
 */

type DepartNode struct {
	ID           int    `gorm:"primary_key;AUTO_INCREMENT"`
	PID          int    `"gorm:type:int(100);not null" json:"nodelocate"`
	ParentDepart string `gorm:"type:varchar(100);not null" json:"parentdepart"`
	Depart       string `gorm:"type:varchar(100);not null" json:"depart"`
	NodeLocate   int    `"gorm:type:int(100);not null" json:"nodelocate"`
	//根节点为0,普通节点为1
}
type MachineNode struct {
	DepartId    int    `"gorm:type:int(100);not null" json:"departid"`
	MachineUUID string `gorm:"type:varchar(100);not null" json:"machineuuid"`
} // kylin/serve/opensource/ops/wanghao/dfsagdasgs

type MachineTreeNode struct {
	Label    string             `json:"label"`
	Id       int                `json:"id"`
	Pid      int                `json:"pid"`
	Children []*MachineTreeNode `json:"children"`
}

type MachineInfo struct {
	Uuid []string `json:"label"`
}