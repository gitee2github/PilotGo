// 插件系统对外提供的api

package pluginapi

import (
	"fmt"

	"github.com/gin-gonic/gin"
	"openeuler.org/PilotGo/PilotGo/pkg/app/server/agentmanager"
	"openeuler.org/PilotGo/PilotGo/pkg/logger"
	"openeuler.org/PilotGo/PilotGo/pkg/utils"
	"openeuler.org/PilotGo/PilotGo/pkg/utils/response"

	"gitee.com/openeuler/PilotGo-plugins/sdk/common"
)

// 检查plugin接口调用权限
func AuthCheck(c *gin.Context) {
	// TODO
	c.Next()
}

type RunResult struct {
	*utils.CmdResult
	MachineUUID string
	MachineIP   string
}

// 远程运行脚本
func RunCommandHandler(c *gin.Context) {
	logger.Debug("process get agent request")
	// uuid := c.Query("uuid")
	// script := c.Query("command")

	// ttcode
	fmt.Println("\033[32mc.request.headers\033[0m: ", c.Request.Header)
	fmt.Println("\033[32mc.request.body\033[0m: ", c.Request.Body)

	d := &struct {
		Batch   *common.Batch `json:"batch"`
		Command string        `json:"command"`
	}{}
	err := c.ShouldBind(d)
	if err != nil {
		logger.Debug("bind batch param error:%s", err)

		response.Fail(c, nil, "parameter error")
		return
	}

	// ttcode
	fmt.Println("\033[32md\033[0m: ", d)
	fmt.Println("\033[32md.batch\033[0m: ", d.Batch)

	logger.Debug("run command on agents :%v", d.Batch.MachineUUIDs)

	result := []*RunResult{}

	if d.Batch.MachineUUIDs != nil {
		for _, uuid := range d.Batch.MachineUUIDs {
			// TODO: support batch
			agent := agentmanager.GetAgent(uuid)
			if agent != nil {
				data, err := agent.RunCommand(d.Command)
				if err != nil {
					logger.Error("run command error, agent:%s, command:%s", uuid, d.Command)
					response.Fail(c, nil, err.Error())
					continue
				}
				logger.Debug("run command on agent result:%v", data)

				result = append(result, &RunResult{
					CmdResult:   data,
					MachineUUID: uuid,
					MachineIP:   agent.IP,
				})
				// response.Success(c, data, "")
				// return
			} else {
				logger.Warn("unknown agent:%s", uuid)
			}
		}

		response.Success(c, result, "")
		return
	} else {
		response.Fail(c, nil, "empty machine uuids")
	}
}

// 远程运行脚本
func RunScriptHandler(c *gin.Context) {
	logger.Debug("process get agent request")

	// ttcode
	fmt.Println("\033[32mc.request.headers\033[0m: ", c.Request.Header)
	fmt.Println("\033[32mc.request.body\033[0m: ", c.Request.Body)

	d := &struct {
		Batch  *common.Batch `json:"batch"`
		Script string        `json:"script"`
		Params []string      `json:"params"`
	}{}
	err := c.ShouldBind(d)
	if err != nil {
		logger.Debug("bind batch param error:%s", err)

		response.Fail(c, nil, "parameter error")
		return
	}

	// ttcode
	fmt.Println("\033[32md\033[0m: ", len(d.Script), len(d.Params))
	fmt.Println("\033[32md.batch\033[0m: ", d.Batch)

	logger.Debug("run script on agents :%v", d.Batch.MachineUUIDs)

	result := []*RunResult{}

	if d.Batch.MachineUUIDs != nil {
		for _, uuid := range d.Batch.MachineUUIDs {
			// TODO: support batch
			agent := agentmanager.GetAgent(uuid)
			if agent != nil {
				data, err := agent.RunScript(d.Script, d.Params)
				if err != nil {
					logger.Error("run command error, agent:%s, command:%s", uuid, d.Script)
					response.Fail(c, nil, err.Error())
					continue
				}
				logger.Debug("run command on agent result:%v", data)

				result = append(result, &RunResult{
					CmdResult:   data,
					MachineUUID: uuid,
					MachineIP:   agent.IP,
				})
				// response.Success(c, data, "")
				// return
			} else {
				logger.Warn("unknown agent:%s", uuid)
			}
		}

		response.Success(c, result, "")
		return
	} else {
		response.Fail(c, nil, "empty machine uuids")
	}
}
