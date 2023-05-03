/******************************************************************************
 * Copyright (c) KylinSoft Co., Ltd.2021-2022. All rights reserved.
 * PilotGo is licensed under the Mulan PSL v2.
 * You can use this software accodring to the terms and conditions of the Mulan PSL v2.
 * You may obtain a copy of Mulan PSL v2 at:
 *     http://license.coscl.org.cn/MulanPSL2
 * THIS SOFTWARE IS PROVIDED ON AN 'AS IS' BASIS, WITHOUT WARRANTIES OF ANY KIND,
 * EITHER EXPRESS OR IMPLIED, INCLUDING BUT NOT LIMITED TO NON-INFRINGEMENT, MERCHANTABILITY OR FIT FOR A PARTICULAR PURPOSE.
 * See the Mulan PSL v2 for more details.
 * Author: zhanghan
 * Date: 2022-06-02 10:25:52
 * LastEditTime: 2022-06-02 16:16:10
 * Description: depart info service
 ******************************************************************************/
package service

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"openeuler.org/PilotGo/PilotGo/pkg/app/server/dao"
	"openeuler.org/PilotGo/PilotGo/pkg/app/server/model"
	"openeuler.org/PilotGo/PilotGo/pkg/global"
)

// 返回全部的部门指针数组
func Returnptrchild(depart []model.DepartNode) (ptrchild []*model.DepartTreeNode, deptRoot model.DepartTreeNode) {
	departnode := make([]model.DepartTreeNode, 0)
	ptrchild = make([]*model.DepartTreeNode, 0)

	for _, value := range depart {
		if value.NodeLocate == 0 {
			deptRoot = model.DepartTreeNode{
				Label: value.Depart,
				Id:    value.ID,
				Pid:   0,
			}
		} else {
			departnode = append(departnode, model.DepartTreeNode{
				Label: value.Depart,
				Id:    value.ID,
				Pid:   value.PID,
			})
		}

	}
	ptrchild = append(ptrchild, &deptRoot)
	var a *model.DepartTreeNode
	for key := range departnode {
		a = &departnode[key]
		ptrchild = append(ptrchild, a)
	}
	return ptrchild, deptRoot
}

// 生成部门树
func MakeTree(node *model.DepartTreeNode, ptrchild []*model.DepartTreeNode) {
	childs := findchild(node, ptrchild)
	for _, value := range childs {
		node.Children = append(node.Children, value)
		if IsChildExist(value, ptrchild) {
			MakeTree(value, ptrchild)
		}
	}
}

// 返回节点的子节点
func findchild(node *model.DepartTreeNode, ptrchild []*model.DepartTreeNode) (ret []*model.DepartTreeNode) {
	for _, value := range ptrchild {
		if node.Id == value.Pid {
			ret = append(ret, value)
		}
	}
	return
}

// 判断是否存在子节点
func IsChildExist(node *model.DepartTreeNode, ptrchild []*model.DepartTreeNode) bool {
	for _, child := range ptrchild {
		if node.Id == child.Pid {
			return true
		}
	}
	return false
}

func LoopTree(node *model.DepartTreeNode, ID int, res **model.DepartTreeNode) {
	if node.Children != nil {
		for _, value := range node.Children {
			if value.Id == ID {
				*res = value
			}

			LoopTree(value, ID, res)

		}

	}
}

func DeleteDepartNode(DepartInfo []model.DepartNode, departid int) {
	needdelete := make([]int, 0)
	needdelete = append(needdelete, departid)
	for _, value := range DepartInfo {
		needdelete = append(needdelete, value.ID)
	}

	for {
		if len(needdelete) == 0 {
			break
		}
		dao.Deletedepartdata(needdelete)
		str := fmt.Sprintf("%d", needdelete[0])
		needdelete = needdelete[1:]
		dao.Insertdepartlist(needdelete, str)
	}
}

// 获取部门下所有机器列表
func MachineList(DepId int) ([]model.Res, error) {
	var departId []int
	ReturnSpecifiedDepart(DepId, &departId)
	departId = append(departId, DepId)
	machinelist1, err := dao.MachineList(departId)
	if err != nil {
		return machinelist1, err
	}
	return machinelist1, nil
}

func Dept(tmp int) (*model.DepartTreeNode, error) {
	depart := dao.DepartStore()
	ptrchild, departRoot := Returnptrchild(depart)
	MakeTree(&departRoot, ptrchild)
	node := &departRoot
	var d *model.DepartTreeNode
	if node.Id != tmp {
		LoopTree(node, tmp, &d)
		node = d
	}
	if node == nil {
		return nil, errors.New("部门ID有误")
	}
	return node, nil
}

func DepartInfo() (*model.DepartTreeNode, error) {
	depart := dao.DepartStore()
	if len(depart) == 0 {
		return nil, errors.New("当前无部门节点")
	}
	ptrchild, departRoot := Returnptrchild(depart)
	MakeTree(&departRoot, ptrchild)
	return &departRoot, nil
}

func AddDepart(newDepart *model.AddDepart) error {
	pid := newDepart.ParentID
	parentDepart := newDepart.ParentDepart
	depart := newDepart.DepartName

	if !dao.IsDepartIDExist(pid) {
		return errors.New("部门PID有误,数据库中不存在该部门PID")
	}

	departNode := model.DepartNode{
		PID:          pid,
		ParentDepart: parentDepart,
		Depart:       depart,
	}
	if dao.IsDepartNodeExist(parentDepart, depart) {
		return errors.New("该部门节点已存在")
	}
	ParentDepartExistBool, err := dao.IsParentDepartExist(parentDepart)
	if err != nil {
		return err
	}
	if len(parentDepart) != 0 && !ParentDepartExistBool {
		return errors.New("该部门上级部门不存在")
	}
	if len(depart) == 0 {
		return errors.New("部门节点不能为空")
	} else if len(parentDepart) == 0 {
		if dao.IsRootExist() {
			return errors.New("已存在根节点,即组织名称")
		} else {
			departNode.NodeLocate = global.Departroot
			if dao.AddDepart(global.PILOTGO_DB, &departNode) != nil {
				return errors.New("部门节点添加失败")
			}
		}
	} else {
		departNode.NodeLocate = global.DepartUnroot
		if dao.AddDepart(global.PILOTGO_DB, &departNode) != nil {
			return errors.New("部门节点添加失败")
		}
	}
	return nil
}

func DeleteDepartData(DelDept *model.DeleteDepart) error {
	if !dao.IsDepartIDExist(DelDept.DepartID) {
		return errors.New("不存在该机器")
	}
	macli, err := dao.MachineStore(DelDept.DepartID)
	if err != nil {
		return err
	}
	for _, mac := range macli {
		err := dao.ModifyMachineDepart2(mac.ID, global.UncateloguedDepartId)
		if err != nil {
			return err
		}
	}
	for _, depart := range dao.SubDepartId(DelDept.DepartID) {
		machine, err := dao.MachineStore(depart)
		if err != nil {
			return err
		}
		for _, m := range machine {
			err := dao.ModifyMachineDepart2(m.ID, global.UncateloguedDepartId)
			if err != nil {
				return err
			}
		}
	}

	DepartInfo := dao.Pid2Depart(DelDept.DepartID)
	DeleteDepartNode(DepartInfo, DelDept.DepartID)
	err = dao.DelUser(DelDept.DepartID)
	if err != nil {
		return err
	}
	return nil
}

func UpdateDepart(DepartID int, DepartName string) error {
	dao.UpdateDepart(DepartID, DepartName)
	dao.UpdateParentDepart(DepartID, DepartName)
	return nil
}

func ModifyMachineDepart(MachineID string, DepartID int) error {
	Ids := strings.Split(MachineID, ",")
	ResIds := make([]int, len(Ids))
	for index, val := range Ids {
		ResIds[index], _ = strconv.Atoi(val)
	}
	for _, id := range ResIds {
		err := dao.ModifyMachineDepart(id, DepartID)
		if err != nil {
			return err
		}
	}
	return nil
}
