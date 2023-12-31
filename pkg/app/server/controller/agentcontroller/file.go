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
 * Date: 2022-05-26 10:25:52
 * LastEditTime: 2023-07-11 19:25:23
 * Description: agent config file handler
 ******************************************************************************/

package agentcontroller

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"openeuler.org/PilotGo/PilotGo/pkg/app/server/agentmanager"
	"openeuler.org/PilotGo/PilotGo/pkg/app/server/dao"
	"openeuler.org/PilotGo/PilotGo/pkg/app/server/service"
	fileservice "openeuler.org/PilotGo/PilotGo/pkg/app/server/service/file"
	"openeuler.org/PilotGo/PilotGo/pkg/logger"
	"openeuler.org/PilotGo/PilotGo/pkg/utils/response"
)

func ReadFile(c *gin.Context) {
	uuid := c.Query("uuid")
	agent := agentmanager.GetAgent(uuid)
	if agent == nil {
		response.Fail(c, nil, "获取uuid失败!")
		return
	}

	filepath := c.Query("file")
	result, Err, err := agent.ReadFile(filepath)
	if err != nil {
		response.Fail(c, nil, Err)
		return
	}
	response.Success(c, gin.H{"file": result}, "Success")
}

func GetAgentRepo(c *gin.Context) {
	uuid := c.Query("uuid")
	agent := agentmanager.GetAgent(uuid)
	if agent == nil {
		response.Fail(c, nil, "获取uuid失败!")
		return
	}

	repos, Err, err := agent.GetRepoSource()
	if len(Err) != 0 || err != nil {
		response.Fail(c, nil, Err)
		return
	}
	response.Success(c, repos, "获取到repo源")
}

func FileBroadcastToAgents(c *gin.Context) {
	var fb fileservice.FileBroadcast
	c.Bind(&fb)

	batchIds := fb.BatchId
	UUIDs := dao.BatchIds2UUIDs(batchIds)

	path := fb.Path
	filename := fb.FileName
	text := fb.Text

	if len(path) == 0 {
		response.Fail(c, nil, "路径为空，请检查配置文件路径")
		return
	}
	if len(filename) == 0 {
		response.Fail(c, nil, "文件名为空，请检查配置文件名字")
		return
	}
	if len(text) == 0 {
		response.Fail(c, nil, "文件内容为空，请重新检查文件内容")
		return
	}
	logParent := dao.AgentLogParent{
		UserName:   fb.User,
		DepartName: fb.UserDept,
		Type:       service.LogTypeBroadcast,
	}
	logParentId, err := dao.ParentAgentLog(logParent)
	if err != nil {
		logger.Error(err.Error())
	}
	StatusCodes := make([]string, 0)

	for _, uuid := range UUIDs {
		agent := agentmanager.GetAgent(uuid)
		if agent == nil {
			log := dao.AgentLog{
				LogParentID:     logParentId,
				IP:              "", // TODO
				OperationObject: filename,
				Action:          service.BroadcastFile,
				StatusCode:      http.StatusBadRequest,
				Message:         "获取uuid失败",
			}
			if dao.AgentLogMessage(log) != nil {
				logger.Error(err.Error())
			}

			StatusCodes = append(StatusCodes, strconv.Itoa(http.StatusBadRequest))
			continue
		}

		_, Err, err := agent.UpdateFile(path, filename, text)
		if len(Err) != 0 || err != nil {

			log := dao.AgentLog{
				LogParentID:     logParentId,
				IP:              agent.IP,
				OperationObject: filename,
				Action:          service.BroadcastFile,
				StatusCode:      http.StatusBadRequest,
				Message:         Err,
			}
			if dao.AgentLogMessage(log) != nil {
				logger.Error(err.Error())
			}

			StatusCodes = append(StatusCodes, strconv.Itoa(http.StatusBadRequest))
			continue
		} else {
			log := dao.AgentLog{
				LogParentID:     logParentId,
				IP:              agent.IP,
				OperationObject: filename,
				Action:          service.BroadcastFile,
				StatusCode:      http.StatusOK,
				Message:         "配置文件下发成功",
			}
			if dao.AgentLogMessage(log) != nil {
				logger.Error(err.Error())
			}

			StatusCodes = append(StatusCodes, strconv.Itoa(http.StatusOK))
		}
	}
	status := service.BatchActionStatus(StatusCodes)
	if dao.UpdateParentAgentLog(logParentId, status) != nil {
		logger.Error(err.Error())
	}
	if ok := service.ActionStatus(StatusCodes); !ok {
		response.Fail(c, nil, "配置文件下发失败")
		return
	}
	response.Success(c, nil, "配置文件下发完成!")
}
