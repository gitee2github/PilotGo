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
 * Date: 2022-02-23 17:46:13
 * LastEditTime: 2022-03-02 13:21:42
 * Description: provide agent log manager functions.
 ******************************************************************************/
package model

import (
	"time"
)

type AgentLogParent struct {
	ID        int `gorm:"primary_key;AUTO_INCREMENT" json:"id"`
	CreatedAt time.Time
	UserName  string     `json:"userName"`
	Type      string     `json:"type"`
	Status    string     `json:"status"`
	AgentLogs []AgentLog `gorm:"ForeignKey:LogParentID;AssociationForeignKey:ID" json:"agent_log"`
}

type AgentLog struct {
	ID              int `gorm:"primary_key;AUTO_INCREMENT" json:"id"`
	LogParentID     int `gorm:"index" json:"logparent_id"`
	LogParent       AgentLogParent
	IP              string `json:"ip"`
	StatusCode      int    `json:"code"`
	OperationObject string `json:"object"`
	Action          string `json:"action"`
	Message         string `json:"message"`
}

const (
	RPMInstall     = "软件包安装"
	RPMRemove      = "软件包卸载"
	SysctlChange   = "修改内核参数"
	ServiceRestart = "重启服务"
	ServiceStop    = "关闭服务"
	ServiceStart   = "开启服务"
)
