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
 * Date: 2022-02-18 02:33:55
 * LastEditTime: 2023-07-11 16:52:58
 * Description: socket server's agentmanager
 ******************************************************************************/
package agentmanager

import (
	"fmt"
	"net"
	"strconv"
	"time"

	"github.com/google/uuid"
	"openeuler.org/PilotGo/PilotGo/pkg/app/agent/global"
	"openeuler.org/PilotGo/PilotGo/pkg/app/server/dao"
	"openeuler.org/PilotGo/PilotGo/pkg/logger"
	"openeuler.org/PilotGo/PilotGo/pkg/utils"
	pnet "openeuler.org/PilotGo/PilotGo/pkg/utils/message/net"
	"openeuler.org/PilotGo/PilotGo/pkg/utils/message/protocol"
	"openeuler.org/PilotGo/PilotGo/pkg/utils/os/common"
)

type AgentMessageHandler func(*Agent, *protocol.Message) error

var WARN_MSG chan interface{}

type Agent struct {
	UUID             string
	Version          string
	IP               string
	conn             net.Conn
	MessageProcesser *protocol.MessageProcesser
	messageChan      chan *protocol.Message
}

// 通过给定的conn连接初始化一个agent并启动监听
func NewAgent(conn net.Conn) (*Agent, error) {
	agent := &Agent{
		UUID:             "agent",
		conn:             conn,
		MessageProcesser: protocol.NewMessageProcesser(),
		messageChan:      make(chan *protocol.Message, 50),
	}

	go func(agent *Agent) {
		for {
			msg := <-agent.messageChan
			logger.Debug("send message:%s", msg.String())
			pnet.SendBytes(agent.conn, protocol.TlvEncode(msg.Encode()))
		}
	}(agent)

	go func(agent *Agent) {
		agent.startListen()
	}(agent)

	if err := agent.Init(); err != nil {
		return nil, err
	}

	return agent, nil
}

func (a *Agent) bindHandler(t int, f AgentMessageHandler) {
	a.MessageProcesser.BindHandler(t, func(c protocol.MessageContext, msg *protocol.Message) error {
		return f(c.(*Agent), msg)
	})
}

func (a *Agent) startListen() {
	defer func() {
		if err := recover(); err != nil {
			logger.Error("server processor panic error:%s", err.(error).Error())
			a.conn.Close()
		}
	}()

	readBuff := []byte{}
	for {
		buff := make([]byte, 1024)
		n, err := a.conn.Read(buff)
		if err != nil {
			err := dao.MachineStatusToOffline(a.UUID)
			if err != nil {
				logger.Error("update machine status failed: %s", err.Error())
			}
			DeleteAgent(a.UUID)
			str := "agent机器" + a.IP + "已断开连接"
			logger.Warn("agent %s disconnected, ip:%s", a.UUID, a.IP)
			WARN_MSG <- str
			return
		}
		readBuff = append(readBuff, buff[:n]...)

		// 切割frame
		i, f := protocol.TlvDecode(&readBuff)
		if i != 0 {
			readBuff = readBuff[i:]
			go func(a *Agent, f *[]byte) {
				msg := protocol.ParseMessage(*f)
				a.MessageProcesser.ProcessMessage(a, msg)
			}(a, f)
		}
	}
}

// 远程获取agent端的信息进行初始化
func (a *Agent) Init() error {
	// TODO: 此处绑定所有的消息处理函数
	a.bindHandler(protocol.Heartbeat, func(a *Agent, msg *protocol.Message) error {
		logger.Info("process heartbeat from processor, remote addr:%s, data:%s",
			a.conn.RemoteAddr().String(), msg.String())
		return nil
	})
	a.bindHandler(protocol.FileMonitor, func(a *Agent, msg *protocol.Message) error {
		logger.Info("process file monitor from processor:%s", msg.String())
		WARN_MSG <- msg.Data.(string)
		return nil
	})

	a.bindHandler(protocol.AgentInfo, func(a *Agent, msg *protocol.Message) error {
		logger.Info("process heartbeat from processor, remote addr:%s, data:%s",
			a.conn.RemoteAddr().String(), msg.String())
		return nil
	})

	a.bindHandler(protocol.ConfigFileMonitor, func(a *Agent, msg *protocol.Message) error {
		logger.Info("remote addr:%s,process config file monitor from processor:%s",
			a.conn.RemoteAddr().String(), msg.String())
		ConfigMessageInfo(msg.Data)
		return nil
	})

	data, err := a.AgentInfo()
	if err != nil {
		logger.Error("fail to get agent info, address:%s", a.conn.RemoteAddr().String())
	}

	a.UUID = data.AgentUUID
	a.IP = data.IP
	a.Version = data.AgentVersion

	return nil
}

// 远程在agent上运行shell命令
func (a *Agent) RunCommand(cmd string) (*utils.CmdResult, error) {
	msg := &protocol.Message{
		UUID: uuid.New().String(),
		Type: protocol.RunCommand,
		Data: struct {
			Command string
		}{
			Command: cmd,
		},
	}

	resp_message, err := a.sendMessage(msg, true, 0)
	if err != nil {
		logger.Error("failed to run command on agent")
		return nil, err
	}

	result := &utils.CmdResult{}
	err = resp_message.BindData(result)
	if err != nil {
		return nil, err
	}
	return result, nil
}

// 远程在agent上运行脚本文件
func (a *Agent) RunScript(script string, params []string) (*utils.CmdResult, error) {
	msg := &protocol.Message{
		UUID: uuid.New().String(),
		Type: protocol.RunScript,
		Data: struct {
			Script string
			Params []string
		}{
			Script: script,
			Params: params,
		},
	}

	resp_message, err := a.sendMessage(msg, true, 0)
	if err != nil {
		logger.Error("failed to run script on agent")
		return nil, err
	}

	result := &utils.CmdResult{}
	err = resp_message.BindData(result)
	if err != nil {
		return nil, err
	}
	return result, nil
}

// TODO: err未发挥作用
func (a *Agent) sendMessage(msg *protocol.Message, wait bool, timeout time.Duration) (*protocol.Message, error) {
	logger.Debug("send message:%s", msg.String())

	if msg.UUID == "" {
		msg.UUID = uuid.New().String()
	}
	if wait {
		waitChan := make(chan *protocol.Message)
		a.MessageProcesser.WaitMap.Store(msg.UUID, waitChan)

		// send message to data send channel
		a.messageChan <- msg

		// wail for response
		data := <-waitChan
		return data, nil
	}

	// just send message to channel
	a.messageChan <- msg
	return nil, nil
}

type AgentInfo struct {
	AgentVersion string `mapstructure:"agent_version"`
	AgentUUID    string `mapstructure:"agent_uuid"`
	IP           string `mapstructure:"IP"`
}

// 远程获取agent端的系统信息
func (a *Agent) AgentInfo() (*AgentInfo, error) {
	msg := &protocol.Message{
		UUID: uuid.New().String(),
		Type: protocol.AgentInfo,
	}

	resp_message, err := a.sendMessage(msg, true, 0)
	if err != nil {
		logger.Error("failed to run script on agent")
		return nil, err
	}

	if resp_message.Status == -1 || resp_message.Error != "" {
		logger.Error("failed to run script on agent: %s", resp_message.Error)
		return nil, fmt.Errorf(resp_message.Error)
	}

	info := &AgentInfo{}
	err = resp_message.BindData(info)
	if err != nil {
		logger.Error("bind AgentInfo data error:", err)
		return nil, err
	}

	return info, nil
}

// 远程获取agent端的系统信息
func (a *Agent) GetOSInfo() (*common.SystemInfo, error) {
	msg := &protocol.Message{
		UUID: uuid.New().String(),
		Type: protocol.OsInfo,
		Data: struct{}{},
	}

	resp_message, err := a.sendMessage(msg, true, 0)
	if err != nil {
		logger.Error("failed to run script on agent")
		return nil, err
	}

	if resp_message.Status == -1 || resp_message.Error != "" {
		logger.Error("failed to run script on agent: %s", resp_message.Error)
		return nil, fmt.Errorf(resp_message.Error)
	}

	info := &common.SystemInfo{}
	err = resp_message.BindData(info)
	if err != nil {
		logger.Error("bind GetOSInfo data error:", err)
		return nil, err
	}
	return info, nil
}

// 远程获取agent端的CPU信息
func (a *Agent) GetCPUInfo() (*common.CPUInfo, error) {
	msg := &protocol.Message{
		UUID: uuid.New().String(),
		Type: protocol.CPUInfo,
		Data: struct{}{},
	}

	resp_message, err := a.sendMessage(msg, true, 0)
	if err != nil {
		logger.Error("failed to run script on agent")
		return nil, err
	}

	if resp_message.Status == -1 || resp_message.Error != "" {
		logger.Error("failed to run script on agent: %s", resp_message.Error)
		return nil, fmt.Errorf(resp_message.Error)
	}

	info := &common.CPUInfo{}
	err = resp_message.BindData(info)
	if err != nil {
		logger.Error("bind GetCPUInfo data error:", err)
		return nil, err
	}
	return info, nil
}

// 远程获取agent端的内存信息
func (a *Agent) GetMemoryInfo() (*common.MemoryConfig, error) {
	msg := &protocol.Message{
		UUID: uuid.New().String(),
		Type: protocol.MemoryInfo,
		Data: struct{}{},
	}

	resp_message, err := a.sendMessage(msg, true, 0)
	if err != nil {
		logger.Error("failed to run script on agent: %s", err.Error())
		return nil, err
	}

	if resp_message.Status == -1 || resp_message.Error != "" {
		logger.Error("failed to run script on agent: %s", resp_message.Error)
		return nil, fmt.Errorf(resp_message.Error)
	}

	info := &common.MemoryConfig{}
	err = resp_message.BindData(info)
	if err != nil {
		logger.Error("bind GetMemoryInfo data error:", err)
		return nil, err
	}
	return info, nil
}

// 远程获取agent端的内核信息
func (a *Agent) GetSysctlInfo() (*map[string]string, error) {
	msg := &protocol.Message{
		UUID: uuid.New().String(),
		Type: protocol.SysctlInfo,
		Data: struct{}{},
	}

	resp_message, err := a.sendMessage(msg, true, 0)
	if err != nil {
		logger.Error("failed to run script on agent")
		return nil, err
	}

	if resp_message.Status == -1 || resp_message.Error != "" {
		logger.Error("failed to run script on agent: %s", resp_message.Error)
		return nil, fmt.Errorf(resp_message.Error)
	}

	info := &map[string]string{}
	err = resp_message.BindData(info)
	if err != nil {
		logger.Error("bind GetSysctlInfo data error:", err)
		return nil, err
	}
	return info, nil
}

// 临时修改agent端系统参数
func (a *Agent) ChangeSysctl(args string) (string, error) {
	msg := &protocol.Message{
		UUID: uuid.New().String(),
		Type: protocol.SysctlChange,
		Data: args,
	}

	resp_message, err := a.sendMessage(msg, true, 0)
	if err != nil {
		logger.Error("failed to run script on agent")
		return "", err
	}

	if resp_message.Status == -1 || resp_message.Error != "" {
		logger.Error("failed to run script on agent: %s", resp_message.Error)
		return "", fmt.Errorf(resp_message.Error)
	}

	return resp_message.Data.(string), nil
}

// 查看某个内核参数的值
func (a *Agent) SysctlView(args string) (string, error) {
	msg := &protocol.Message{
		UUID: uuid.New().String(),
		Type: protocol.SysctlView,
		Data: args,
	}

	resp_message, err := a.sendMessage(msg, true, 0)
	if err != nil {
		logger.Error("failed to run script on agent")
		return "", err
	}

	if resp_message.Status == -1 || resp_message.Error != "" {
		logger.Error("failed to run script on agent: %s", resp_message.Error)
		return "", fmt.Errorf(resp_message.Error)
	}

	return resp_message.Data.(string), nil
}

// 查看服务列表
func (a *Agent) ServiceList() ([]*common.ListService, error) {
	msg := &protocol.Message{
		UUID: uuid.New().String(),
		Type: protocol.ServiceList,
		Data: struct{}{},
	}

	resp_message, err := a.sendMessage(msg, true, 0)
	if err != nil {
		logger.Error("failed to run script on agent")
		return nil, err
	}

	if resp_message.Status == -1 || resp_message.Error != "" {
		logger.Error("failed to run script on agent: %s", resp_message.Error)
		return nil, fmt.Errorf(resp_message.Error)
	}

	info := &[]*common.ListService{}
	err = resp_message.BindData(info)
	if err != nil {
		logger.Error("bind ServiceList data error:", err)
		return nil, err
	}
	return *info, nil
}

// 查看某个服务的状态
func (a *Agent) ServiceStatus(service string) (string, error) {
	msg := &protocol.Message{
		UUID: uuid.New().String(),
		Type: protocol.ServiceStatus,
		Data: service,
	}

	resp_message, err := a.sendMessage(msg, true, 0)
	if err != nil {
		logger.Error("failed to run script on agent")
		return "", err
	}

	if resp_message.Status == -1 || resp_message.Error != "" {
		logger.Error("failed to run script on agent: %s", resp_message.Error)
		return "", fmt.Errorf(resp_message.Error)
	}

	return resp_message.Data.(string), nil
}

// 重启服务
func (a *Agent) ServiceRestart(service string) (string, string, error) {
	msg := &protocol.Message{
		UUID: uuid.New().String(),
		Type: protocol.ServiceRestart,
		Data: service,
	}

	resp_message, err := a.sendMessage(msg, true, 0)
	if err != nil {
		logger.Error("failed to run script on agent")
		return "", "", err
	}

	if resp_message.Status == -1 || resp_message.Error != "" {
		logger.Error("failed to run script on agent: %s", resp_message.Error)
		return "", resp_message.Error, fmt.Errorf(resp_message.Error)
	}

	return resp_message.Data.(string), resp_message.Error, nil
}

// 关闭服务
func (a *Agent) ServiceStop(service string) (string, string, error) {
	msg := &protocol.Message{
		UUID: uuid.New().String(),
		Type: protocol.ServiceStop,
		Data: service,
	}

	resp_message, err := a.sendMessage(msg, true, 0)
	if err != nil {
		logger.Error("failed to run script on agent")
		return "", "", err
	}

	if resp_message.Status == -1 || resp_message.Error != "" {
		logger.Error("failed to run script on agent: %s", resp_message.Error)
		return "", resp_message.Error, fmt.Errorf(resp_message.Error)
	}

	return resp_message.Data.(string), resp_message.Error, nil
}

// 启动服务
func (a *Agent) ServiceStart(service string) (string, string, error) {
	msg := &protocol.Message{
		UUID: uuid.New().String(),
		Type: protocol.ServiceStart,
		Data: service,
	}

	resp_message, err := a.sendMessage(msg, true, 0)
	if err != nil {
		logger.Error("failed to run script on agent")
		return "", "", err
	}

	if resp_message.Status == -1 || resp_message.Error != "" {
		logger.Error("failed to run script on agent: %s", resp_message.Error)
		return "", resp_message.Error, fmt.Errorf(resp_message.Error)
	}

	return resp_message.Data.(string), resp_message.Error, nil
}

// 获取全部安装的rpm包列表
func (a *Agent) AllRpm() ([]string, error) {
	msg := &protocol.Message{
		UUID: uuid.New().String(),
		Type: protocol.AllRpm,
		Data: struct{}{},
	}

	resp_message, err := a.sendMessage(msg, true, 0)
	if err != nil {
		logger.Error("failed to run script on agent")
		return nil, err
	}

	if resp_message.Status == -1 || resp_message.Error != "" {
		logger.Error("failed to run script on agent: %s", resp_message.Error)
		return nil, fmt.Errorf(resp_message.Error)
	}

	if v, ok := resp_message.Data.([]interface{}); ok {
		result := make([]string, len(v))
		for i, item := range v {
			if str, ok := item.(string); ok {
				result[i] = str
			}
		}
		return result, nil
	}
	return nil, fmt.Errorf("failed to convert interface{} in allrpm")
}

// 获取源软件包名以及源
func (a *Agent) RpmSource(rpm string) (*common.RpmSrc, error) {
	msg := &protocol.Message{
		UUID: uuid.New().String(),
		Type: protocol.RpmSource,
		Data: rpm,
	}

	resp_message, err := a.sendMessage(msg, true, 0)
	if err != nil {
		logger.Error("failed to run script on agent")
		return nil, err
	}

	if resp_message.Status == -1 || resp_message.Error != "" {
		logger.Error("failed to run script on agent: %s", resp_message.Error)
		return nil, fmt.Errorf(resp_message.Error)
	}

	info := &common.RpmSrc{}
	err = resp_message.BindData(info)
	if err != nil {
		logger.Error("bind RpmSource data error:", err)
		return nil, err
	}
	return info, nil
}

// 获取软件包信息
func (a *Agent) RpmInfo(rpm string) (*common.RpmInfo, string, error) {
	msg := &protocol.Message{
		UUID: uuid.New().String(),
		Type: protocol.RpmInfo,
		Data: rpm,
	}

	resp_message, err := a.sendMessage(msg, true, 0)
	if err != nil {
		logger.Error("failed to run script on agent")
		return nil, "", err
	}

	if resp_message.Status == -1 || resp_message.Error != "" {
		logger.Error("failed to run script on agent: %s", resp_message.Error)
		return nil, resp_message.Error, fmt.Errorf(resp_message.Error)
	}

	info := &common.RpmInfo{}
	err = resp_message.BindData(info)
	if err != nil {
		logger.Error("bind RpmInfo data error:", err)
		return nil, "", err
	}
	return info, resp_message.Error, nil
}

// 获取源软件包名以及源
func (a *Agent) InstallRpm(rpm string) (string, string, error) {
	msg := &protocol.Message{
		UUID: uuid.New().String(),
		Type: protocol.InstallRpm,
		Data: rpm,
	}

	resp_message, err := a.sendMessage(msg, true, 0)
	if err != nil {
		logger.Error("failed to run script on agent")
		return "", "", err
	}

	if resp_message.Status == -1 || resp_message.Error != "" {
		logger.Error("failed to run script on agent: %s", resp_message.Error)
		return "", resp_message.Error, fmt.Errorf(resp_message.Error)
	}

	return resp_message.Data.(string), resp_message.Error, nil
}

// 获取源软件包名以及源
func (a *Agent) RemoveRpm(rpm string) (string, string, error) {
	msg := &protocol.Message{
		UUID: uuid.New().String(),
		Type: protocol.RemoveRpm,
		Data: rpm,
	}

	resp_message, err := a.sendMessage(msg, true, 0)
	if err != nil {
		logger.Error("failed to run script on agent")
		return "", "", err
	}

	if resp_message.Status == -1 || resp_message.Error != "" {
		logger.Error("failed to run script on agent: %s", resp_message.Error)
		return "", resp_message.Error, fmt.Errorf(resp_message.Error)
	}

	return resp_message.Data.(string), resp_message.Error, nil
}

// 获取磁盘的使用情况
func (a *Agent) DiskUsage() ([]*common.DiskUsageINfo, error) {
	msg := &protocol.Message{
		UUID: uuid.New().String(),
		Type: protocol.DiskUsage,
		Data: struct{}{},
	}

	resp_message, err := a.sendMessage(msg, true, 0)
	if err != nil {
		logger.Error("failed to run script on agent")
		return nil, err
	}

	if resp_message.Status == -1 || resp_message.Error != "" {
		logger.Error("failed to run script on agent: %s", resp_message.Error)
		return nil, fmt.Errorf(resp_message.Error)
	}

	info := &[]*common.DiskUsageINfo{}
	err = resp_message.BindData(info)
	if err != nil {
		logger.Error("bind DiskUsage data error:", err)
		return nil, err
	}
	return *info, nil
}

// 获取磁盘的IO信息
func (a *Agent) DiskInfo() (*common.DiskIOInfo, error) {
	msg := &protocol.Message{
		UUID: uuid.New().String(),
		Type: protocol.DiskInfo,
		Data: struct{}{},
	}

	resp_message, err := a.sendMessage(msg, true, 0)
	if err != nil {
		logger.Error("failed to run script on agent")
		return nil, err
	}

	if resp_message.Status == -1 || resp_message.Error != "" {
		logger.Error("failed to run script on agent: %s", resp_message.Error)
		return nil, fmt.Errorf(resp_message.Error)
	}

	info := &common.DiskIOInfo{}
	err = resp_message.BindData(info)
	if err != nil {
		logger.Error("bind DiskInfo data error", err)
		return nil, err
	}
	return info, nil
}

/*
挂载磁盘
1.创建挂载磁盘的目录
2.挂载磁盘
*/

func (a *Agent) DiskMount(sourceDisk, destPath string) (string, error) {
	msg := &protocol.Message{
		UUID: uuid.New().String(),
		Type: protocol.DiskMount,
		Data: sourceDisk + "," + destPath,
	}

	resp_message, err := a.sendMessage(msg, true, 0)
	if err != nil {
		logger.Error("failed to run script on agent")
		return err.Error(), err
	}

	if resp_message.Status == -1 || resp_message.Error != "" {
		logger.Error("failed to run script on agent: %s", resp_message.Error)
		return "", fmt.Errorf(resp_message.Error)
	}

	return resp_message.Data.(string), nil
}
func (a *Agent) DiskUMount(diskPath string) (string, error) {
	msg := &protocol.Message{
		UUID: uuid.New().String(),
		Type: protocol.DiskUMount,
		Data: diskPath,
	}

	resp_message, err := a.sendMessage(msg, true, 0)
	if err != nil {
		logger.Error("failed to run script on agent")
		return err.Error(), err
	}

	if resp_message.Status == -1 || resp_message.Error != "" {
		logger.Error("failed to run script on agent: %s", resp_message.Error)
		return "", fmt.Errorf(resp_message.Error)
	}

	return resp_message.Data.(string), nil
}
func (a *Agent) DiskFormat(fileType, diskPath string) (string, error) {
	msg := &protocol.Message{
		UUID: uuid.New().String(),
		Type: protocol.DiskFormat,
		Data: fileType + "," + diskPath,
	}

	resp_message, err := a.sendMessage(msg, true, 0)
	if err != nil {
		logger.Error("failed to run script on agent")
		return "", err
	}

	if resp_message.Status == -1 || resp_message.Error != "" {
		logger.Error("failed to run script on agent: %s", resp_message.Error)
		return "", fmt.Errorf(resp_message.Error)
	}

	return resp_message.Data.(string), nil
}

// 获取当前TCP网络连接信息
func (a *Agent) NetTCP() (*common.NetConnect, error) {
	msg := &protocol.Message{
		UUID: uuid.New().String(),
		Type: protocol.NetTCP,
		Data: struct{}{},
	}

	resp_message, err := a.sendMessage(msg, true, 0)
	if err != nil {
		logger.Error("failed to run script on agent")
		return nil, err
	}

	if resp_message.Status == -1 || resp_message.Error != "" {
		logger.Error("failed to run script on agent: %s", resp_message.Error)
		return nil, fmt.Errorf(resp_message.Error)
	}

	info := &common.NetConnect{}
	err = resp_message.BindData(info)
	if err != nil {
		logger.Error("bind NetTCP data error:", err)
		return nil, err
	}
	return info, nil
}

// 获取当前UDP网络连接信息
func (a *Agent) NetUDP() (*common.NetConnect, error) {
	msg := &protocol.Message{
		UUID: uuid.New().String(),
		Type: protocol.NetUDP,
		Data: struct{}{},
	}

	resp_message, err := a.sendMessage(msg, true, 0)
	if err != nil {
		logger.Error("failed to run script on agent")
		return nil, err
	}

	if resp_message.Status == -1 || resp_message.Error != "" {
		logger.Error("failed to run script on agent: %s", resp_message.Error)
		return nil, fmt.Errorf(resp_message.Error)
	}

	info := &common.NetConnect{}
	err = resp_message.BindData(info)
	if err != nil {
		logger.Error("bind NetUDP data error:", err)
		return nil, err
	}
	return info, nil
}

// 获取网络读写字节／包的个数
func (a *Agent) NetIOCounter() (*common.IOCnt, error) {
	msg := &protocol.Message{
		UUID: uuid.New().String(),
		Type: protocol.NetIOCounter,
		Data: struct{}{},
	}

	resp_message, err := a.sendMessage(msg, true, 0)
	if err != nil {
		logger.Error("failed to run script on agent")
		return nil, err
	}

	if resp_message.Status == -1 || resp_message.Error != "" {
		logger.Error("failed to run script on agent: %s", resp_message.Error)
		return nil, fmt.Errorf(resp_message.Error)
	}

	info := &common.IOCnt{}
	err = resp_message.BindData(info)
	if err != nil {
		logger.Error("bind NetIOCounter data error:", err)
		return nil, err
	}
	return info, nil
}

// 获取网卡配置
func (a *Agent) NetNICConfig() (*common.NetInterfaceCard, error) {
	msg := &protocol.Message{
		UUID: uuid.New().String(),
		Type: protocol.NetNICConfig,
		Data: struct{}{},
	}

	resp_message, err := a.sendMessage(msg, true, 0)
	if err != nil {
		logger.Error("failed to run script on agent")
		return nil, err
	}

	if resp_message.Status == -1 || resp_message.Error != "" {
		logger.Error("failed to run script on agent: %s", resp_message.Error)
		return nil, fmt.Errorf(resp_message.Error)
	}

	info := &common.NetInterfaceCard{}
	err = resp_message.BindData(info)
	if err != nil {
		logger.Error("bind NetNICConfig data error:", err)
		return nil, err
	}
	return info, nil
}

// 获取当前用户信息
func (a *Agent) CurrentUser() (*common.CurrentUser, error) {
	msg := &protocol.Message{
		UUID: uuid.New().String(),
		Type: protocol.CurrentUser,
		Data: struct{}{},
	}

	resp_message, err := a.sendMessage(msg, true, 0)
	if err != nil {
		logger.Error("failed to run script on agent")
		return nil, err
	}

	if resp_message.Status == -1 || resp_message.Error != "" {
		logger.Error("failed to run script on agent: %s", resp_message.Error)
		return nil, fmt.Errorf(resp_message.Error)
	}

	info := &common.CurrentUser{}
	err = resp_message.BindData(info)
	if err != nil {
		logger.Error("bind CurrentUser data error:", err)
		return nil, err
	}
	return info, nil
}

// 获取所有用户的信息
func (a *Agent) AllUser() ([]*common.AllUserInfo, error) {
	msg := &protocol.Message{
		UUID: uuid.New().String(),
		Type: protocol.AllUser,
		Data: struct{}{},
	}

	resp_message, err := a.sendMessage(msg, true, 0)
	if err != nil {
		logger.Error("failed to run script on agent")
		return nil, err
	}

	if resp_message.Status == -1 || resp_message.Error != "" {
		logger.Error("failed to run script on agent: %s", resp_message.Error)
		return nil, fmt.Errorf(resp_message.Error)
	}

	info := &[]*common.AllUserInfo{}
	err = resp_message.BindData(info)
	if err != nil {
		logger.Error("bind AllUser data error:", err)
		return nil, err
	}
	return *info, nil
}

// 创建新的用户，并新建家目录
func (a *Agent) AddLinuxUser(username, password string) (string, error) {
	msg := &protocol.Message{
		UUID: uuid.New().String(),
		Type: protocol.AddLinuxUser,
		Data: username + "," + password,
	}

	resp_message, err := a.sendMessage(msg, true, 0)
	if err != nil {
		logger.Error("failed to run script on agent")
		return "", err
	}

	if resp_message.Status == -1 || resp_message.Error != "" {
		logger.Error("failed to run script on agent: %s", resp_message.Error)
		return "", fmt.Errorf(resp_message.Error)
	}

	return resp_message.Data.(string), nil
}

// 删除用户
func (a *Agent) DelUser(username string) (string, string, error) {
	msg := &protocol.Message{
		UUID: uuid.New().String(),
		Type: protocol.DelUser,
		Data: username,
	}

	resp_message, err := a.sendMessage(msg, true, 0)
	if err != nil {
		logger.Error("failed to run script on agent")
		return "", "", err
	}

	if resp_message.Status == -1 || resp_message.Error != "" {
		logger.Error("failed to run script on agent: %s", resp_message.Error)
		return "", resp_message.Error, fmt.Errorf(resp_message.Error)
	}

	return resp_message.Data.(string), resp_message.Error, nil
}

// chmod [-R] 权限值 文件名
func (a *Agent) ChangePermission(permission, file string) (string, error) {
	msg := &protocol.Message{
		UUID: uuid.New().String(),
		Type: protocol.ChangePermission,
		Data: permission + "," + file,
	}

	resp_message, err := a.sendMessage(msg, true, 0)
	if err != nil {
		logger.Error("failed to run script on agent")
		return "", err
	}

	if resp_message.Status == -1 || resp_message.Error != "" {
		logger.Error("failed to run script on agent: %s", resp_message.Error)
		return "", fmt.Errorf(resp_message.Error)
	}

	return resp_message.Data.(string), nil
}

// chown [-R] 所有者 文件或目录
func (a *Agent) ChangeFileOwner(user, file string) (string, error) {
	msg := &protocol.Message{
		UUID: uuid.New().String(),
		Type: protocol.ChangeFileOwner,
		Data: user + "," + file,
	}

	resp_message, err := a.sendMessage(msg, true, 0)
	if err != nil {
		logger.Error("failed to run script on agent")
		return "", err
	}

	if resp_message.Status == -1 || resp_message.Error != "" {
		logger.Error("failed to run script on agent: %s", resp_message.Error)
		return "", fmt.Errorf(resp_message.Error)
	}

	return resp_message.Data.(string), nil
}

// 远程获取agent端的内核信息
func (a *Agent) GetAgentOSInfo() (*common.SystemAndCPUInfo, error) {
	msg := &protocol.Message{
		UUID: uuid.New().String(),
		Type: protocol.AgentOSInfo,
		Data: struct{}{},
	}

	resp_message, err := a.sendMessage(msg, true, 0)
	if err != nil {
		logger.Error("failed to run script on agent")
		return nil, err
	}

	if resp_message.Status == -1 || resp_message.Error != "" {
		logger.Error("failed to run script on agent: %s", resp_message.Error)
		return nil, fmt.Errorf(resp_message.Error)
	}

	info := &common.SystemAndCPUInfo{}
	err = resp_message.BindData(info)
	if err != nil {
		logger.Error("bind GetAgentOSInfo data error:", err)
		return nil, err
	}
	return info, nil
}

// 心跳
func (a *Agent) HeartBeat() (string, error) {
	msg := &protocol.Message{
		UUID: uuid.New().String(),
		Type: protocol.Heartbeat,
		Data: "连接正常",
	}

	resp_message, err := a.sendMessage(msg, true, 0)
	if err != nil {
		logger.Error("failed to run script on agent")
		return "", err
	}

	if resp_message.Status == -1 || resp_message.Error != "" {
		logger.Error("failed to run script on agent: %s", resp_message.Error)
		return "", fmt.Errorf(resp_message.Error)
	}

	return resp_message.Data.(string), nil
}

// 获取防火墙配置
func (a *Agent) FirewalldConfig() (*common.FireWalldConfig, string, error) {
	msg := &protocol.Message{
		UUID: uuid.New().String(),
		Type: protocol.FirewalldConfig,
		Data: struct{}{},
	}

	resp_message, err := a.sendMessage(msg, true, 0)
	if err != nil {
		logger.Error("failed to run script on agent")
		return nil, "", err
	}

	if resp_message.Status == -1 || resp_message.Error != "" {
		logger.Error("failed to run script on agent: %s", resp_message.Error)
		return nil, resp_message.Error, fmt.Errorf(resp_message.Error)
	}

	info := &common.FireWalldConfig{}
	err = resp_message.BindData(info)
	if err != nil {
		logger.Error("bind FirewalldConfig data error:", err)
		return nil, resp_message.Error, err
	}
	return info, resp_message.Error, nil
}

// 更改防火墙默认区域
func (a *Agent) FirewalldSetDefaultZone(zone string) (string, string, error) {
	msg := &protocol.Message{
		UUID: uuid.New().String(),
		Type: protocol.FirewalldDefaultZone,
		Data: zone,
	}

	resp_message, err := a.sendMessage(msg, true, 0)
	if err != nil {
		logger.Error("failed to run script on agent")
		return "", "", err
	}

	if resp_message.Status == -1 || resp_message.Error != "" {
		logger.Error("failed to run script on agent: %s", resp_message.Error)
		return "", resp_message.Error, fmt.Errorf(resp_message.Error)
	}

	return resp_message.Data.(string), resp_message.Error, nil
}

// 查看防火墙指定区域配置
func (a *Agent) FirewalldZoneConfig(zone string) (*common.FirewalldCMDList, string, error) {
	msg := &protocol.Message{
		UUID: uuid.New().String(),
		Type: protocol.FirewalldZoneConfig,
		Data: zone,
	}

	resp_message, err := a.sendMessage(msg, true, 0)
	if err != nil {
		logger.Error("failed to run script on agent")
		return nil, "", err
	}

	if resp_message.Status == -1 || resp_message.Error != "" {
		logger.Error("failed to run script on agent: %s", resp_message.Error)
		return nil, resp_message.Error, fmt.Errorf(resp_message.Error)
	}

	info := &common.FirewalldCMDList{}
	err = resp_message.BindData(info)
	if err != nil {
		logger.Error("bind data error:", err)
		return nil, resp_message.Error, err
	}
	return info, resp_message.Error, nil
}

// 添加防火墙服务
func (a *Agent) FirewalldServiceAdd(zone, service string) (string, error) {
	msg := &protocol.Message{
		UUID: uuid.New().String(),
		Type: protocol.FirewalldServiceAdd,
		Data: zone + "," + service,
	}

	resp_message, err := a.sendMessage(msg, true, 0)
	if err != nil {
		logger.Error("failed to run script on agent")
		return "", err
	}

	if resp_message.Status == -1 || resp_message.Error != "" {
		logger.Error("failed to run script on agent: %s", resp_message.Error)
		return resp_message.Error, fmt.Errorf(resp_message.Error)
	}

	return resp_message.Error, nil
}

// 移除防火墙服务
func (a *Agent) FirewalldServiceRemove(zone, service string) (string, error) {
	msg := &protocol.Message{
		UUID: uuid.New().String(),
		Type: protocol.FirewalldServiceRemove,
		Data: zone + "," + service,
	}

	resp_message, err := a.sendMessage(msg, true, 0)
	if err != nil {
		logger.Error("failed to run script on agent")
		return "", err
	}

	if resp_message.Status == -1 || resp_message.Error != "" {
		logger.Error("failed to run script on agent: %s", resp_message.Error)
		return resp_message.Error, fmt.Errorf(resp_message.Error)
	}

	return resp_message.Error, nil
}

// 防火墙添加允许来源地址
func (a *Agent) FirewalldSourceAdd(zone, source string) (string, error) {
	msg := &protocol.Message{
		UUID: uuid.New().String(),
		Type: protocol.FirewalldSourceAdd,
		Data: zone + "," + source,
	}

	resp_message, err := a.sendMessage(msg, true, 0)
	if err != nil {
		logger.Error("failed to run script on agent")
		return "", err
	}

	if resp_message.Status == -1 || resp_message.Error != "" {
		logger.Error("failed to run script on agent: %s", resp_message.Error)
		return resp_message.Error, fmt.Errorf(resp_message.Error)
	}

	return resp_message.Error, nil
}

// 防火墙移除允许来源地址
func (a *Agent) FirewalldSourceRemove(zone, source string) (string, error) {
	msg := &protocol.Message{
		UUID: uuid.New().String(),
		Type: protocol.FirewalldSourceRemove,
		Data: zone + "," + source,
	}

	resp_message, err := a.sendMessage(msg, true, 0)
	if err != nil {
		logger.Error("failed to run script on agent")
		return "", err
	}

	if resp_message.Status == -1 || resp_message.Error != "" {
		logger.Error("failed to run script on agent: %s", resp_message.Error)
		return resp_message.Error, fmt.Errorf(resp_message.Error)
	}

	return resp_message.Error, nil
}

// 重启防火墙
func (a *Agent) FirewalldRestart() (bool, string, error) {
	msg := &protocol.Message{
		UUID: uuid.New().String(),
		Type: protocol.FirewalldRestart,
		Data: struct{}{},
	}

	resp_message, err := a.sendMessage(msg, true, 0)
	if err != nil {
		logger.Error("failed to run script on agent")
		return false, "", err
	}

	if resp_message.Status == -1 || resp_message.Error != "" {
		logger.Error("failed to run script on agent: %s", resp_message.Error)
		return false, resp_message.Error, fmt.Errorf(resp_message.Error)
	}

	return resp_message.Data.(bool), resp_message.Error, nil
}

// 关闭防火墙
func (a *Agent) FirewalldStop() (bool, string, error) {
	msg := &protocol.Message{
		UUID: uuid.New().String(),
		Type: protocol.FirewalldStop,
		Data: struct{}{},
	}

	resp_message, err := a.sendMessage(msg, true, 0)
	if err != nil {
		logger.Error("failed to run script on agent")
		return false, "", err
	}

	if resp_message.Status == -1 || resp_message.Error != "" {
		logger.Error("failed to run script on agent: %s", resp_message.Error)
		return false, resp_message.Error, fmt.Errorf(resp_message.Error)
	}

	return resp_message.Data.(bool), resp_message.Error, nil
}

// 防火墙指定区域添加端口
func (a *Agent) FirewalldZonePortAdd(zone, port, proto string) (string, string, error) {
	msg := &protocol.Message{
		UUID: uuid.New().String(),
		Type: protocol.FirewalldZonePortAdd,
		Data: zone + "," + port + "," + proto,
	}

	resp_message, err := a.sendMessage(msg, true, 0)
	if err != nil {
		logger.Error("failed to run script on agent")
		return "", "", err
	}

	if resp_message.Status == -1 || resp_message.Error != "" {
		logger.Error("failed to run script on agent: %s", resp_message.Error)
		return "", resp_message.Error, fmt.Errorf(resp_message.Error)
	}

	return resp_message.Data.(string), resp_message.Error, nil
}

// 防火墙指定区域删除端口
func (a *Agent) FirewalldZonePortDel(zone, port, proto string) (string, string, error) {
	msg := &protocol.Message{
		UUID: uuid.New().String(),
		Type: protocol.FirewalldZonePortDel,
		Data: zone + "," + port + "," + proto,
	}

	resp_message, err := a.sendMessage(msg, true, 0)
	if err != nil {
		logger.Error("failed to run script on agent")
		return "", "", err
	}

	if resp_message.Status == -1 || resp_message.Error != "" {
		logger.Error("failed to run script on agent: %s", resp_message.Error)
		return "", resp_message.Error, fmt.Errorf(resp_message.Error)
	}

	return resp_message.Data.(string), resp_message.Error, nil
}

// 开启定时任务
func (a *Agent) CronStart(id int, spec string, command string) (string, string, error) {
	msg := &protocol.Message{
		UUID: uuid.New().String(),
		Type: protocol.CronStart,
		Data: strconv.Itoa(id) + "," + spec + "," + command,
	}

	resp_message, err := a.sendMessage(msg, true, 0)
	if err != nil {
		logger.Error("failed to run script on agent")
		return "", "", err
	}

	if resp_message.Status == -1 || resp_message.Error != "" {
		logger.Error("failed to run script on agent: %s", resp_message.Error)
		return "", resp_message.Error, fmt.Errorf(resp_message.Error)
	}

	return resp_message.Data.(string), resp_message.Error, nil
}

// 暂停定时任务
func (a *Agent) CronStopAndDel(id int) (string, error) {
	msg := &protocol.Message{
		UUID: uuid.New().String(),
		Type: protocol.CronStopAndDel,
		Data: strconv.Itoa(id),
	}

	resp_message, err := a.sendMessage(msg, true, 0)
	if err != nil {
		logger.Error("failed to run script on agent")
		return "", err
	}

	if resp_message.Status == -1 || resp_message.Error != "" {
		logger.Error("failed to run script on agent: %s", resp_message.Error)
		return "", fmt.Errorf(resp_message.Error)
	}

	return resp_message.Data.(string), nil
}

// 远程获取agent端的repo文件
func (a *Agent) GetRepoSource() ([]*common.RepoSource, string, error) {
	msg := &protocol.Message{
		UUID: uuid.New().String(),
		Type: protocol.GetRepoSource,
		Data: struct{}{},
	}

	resp_message, err := a.sendMessage(msg, true, 0)
	if err != nil {
		logger.Error("failed to run script on agent")
		return nil, "", err
	}

	if resp_message.Status == -1 || resp_message.Error != "" {
		logger.Error("failed to run script on agent: %s", resp_message.Error)
		return nil, resp_message.Error, fmt.Errorf(resp_message.Error)
	}

	info := &[]*common.RepoSource{}
	err = resp_message.BindData(info)
	if err != nil {
		logger.Error("bind data error:", err)
		return nil, resp_message.Error, err
	}
	return *info, resp_message.Error, nil
}

// 远程获取agent端的网络连接信息
func (a *Agent) GetNetWorkConnectInfo() (*map[string]string, string, error) {
	msg := &protocol.Message{
		UUID: uuid.New().String(),
		Type: protocol.GetNetWorkConnectInfo,
		Data: struct{}{},
	}

	resp_message, err := a.sendMessage(msg, true, 0)
	if err != nil {
		logger.Error("failed to run script on agent")
		return nil, "", err
	}

	if resp_message.Status == -1 || resp_message.Error != "" {
		logger.Error("failed to run script on agent: %s", resp_message.Error)
		return nil, resp_message.Error, fmt.Errorf(resp_message.Error)
	}

	info := &map[string]string{}
	err = resp_message.BindData(info)
	if err != nil {
		logger.Error("bind GetSysctlInfo data error:", err)
		return nil, resp_message.Error, err
	}
	return info, resp_message.Error, nil
}

// 获取agent的基础网络配置
func (a *Agent) GetNetWorkConnInfo() (*common.NetworkConfig, string, error) {
	msg := &protocol.Message{
		UUID: uuid.New().String(),
		Type: protocol.GetNetWorkConnInfo,
		Data: struct{}{},
	}

	resp_message, err := a.sendMessage(msg, true, 0)
	if err != nil {
		logger.Error("failed to run script on agent")
		return nil, "", err
	}

	if resp_message.Status == -1 || resp_message.Error != "" {
		logger.Error("failed to run script on agent: %s", resp_message.Error)
		return nil, resp_message.Error, fmt.Errorf(resp_message.Error)
	}

	info := &common.NetworkConfig{}
	err = resp_message.BindData(info)
	if err != nil {
		logger.Error("bind GetNetWorkConnInfo data error:", err)
		return nil, resp_message.Error, err
	}
	return info, resp_message.Error, nil
}

// 获取网卡名字
func (a *Agent) GetNICName() (string, string, error) {
	msg := &protocol.Message{
		UUID: uuid.New().String(),
		Type: protocol.GetNICName,
		Data: struct{}{},
	}

	resp_message, err := a.sendMessage(msg, true, 0)
	if err != nil {
		logger.Error("failed to run script on agent")
		return "", "", err
	}

	if resp_message.Status == -1 || resp_message.Error != "" {
		logger.Error("failed to run script on agent: %s", resp_message.Error)
		return "", resp_message.Error, fmt.Errorf(resp_message.Error)
	}

	return resp_message.Data.(string), resp_message.Error, nil
}

// 重启网卡配置
func (a *Agent) RestartNetWork(NIC string) (string, error) {
	msg := &protocol.Message{
		UUID: uuid.New().String(),
		Type: protocol.RestartNetWork,
		Data: NIC,
	}

	resp_message, err := a.sendMessage(msg, true, 0)
	if err != nil {
		logger.Error("failed to run script on agent")
		return "", err
	}

	if resp_message.Status == -1 || resp_message.Error != "" {
		logger.Error("failed to run script on agent: %s", resp_message.Error)
		return resp_message.Error, fmt.Errorf(resp_message.Error)
	}

	return resp_message.Error, nil
}

// 查看配置文件内容
func (a *Agent) ReadFile(filepath string) (string, string, error) {
	msg := &protocol.Message{
		UUID: uuid.New().String(),
		Type: protocol.ReadFile,
		Data: filepath,
	}

	resp_message, err := a.sendMessage(msg, true, 0)
	if err != nil {
		logger.Error("failed to run script on agent")
		return "", "", err
	}

	if resp_message.Status == -1 || resp_message.Error != "" {
		logger.Error("failed to run script on agent: %s", resp_message.Error)
		return "", resp_message.Error, fmt.Errorf(resp_message.Error)
	}

	return resp_message.Data.(string), resp_message.Error, nil
}

// 更新配置文件
func (a *Agent) UpdateFile(filepath string, filename string, text string) (*common.UpdateFile, string, error) {
	updatefile := common.UpdateFile{
		FilePath: filepath,
		FileName: filename,
		FileText: text,
	}
	msg := &protocol.Message{
		UUID: uuid.New().String(),
		Type: protocol.EditFile,
		Data: updatefile,
	}

	resp_message, err := a.sendMessage(msg, true, 0)
	if err != nil {
		logger.Error("failed to run script on agent")
		return nil, "", err
	}

	if resp_message.Status == -1 || resp_message.Error != "" {
		logger.Error("failed to run script on agent: %s", resp_message.Error)
		return nil, resp_message.Error, fmt.Errorf(resp_message.Error)
	}

	info := &common.UpdateFile{}
	err = resp_message.BindData(info)
	if err != nil {
		logger.Error("bind UpdateFile data error:", err)
		return nil, resp_message.Error, err
	}
	return info, resp_message.Error, nil
}

// 远程获取agent端的时间信息
func (a *Agent) GetTimeInfo() (string, error) {
	msg := &protocol.Message{
		UUID: uuid.New().String(),
		Type: protocol.AgentTime,
		Data: struct{}{},
	}
	resp_message, err := a.sendMessage(msg, true, 0)
	if err != nil {
		logger.Error("failed to get time on agent")
		return "", err
	}

	if resp_message.Status == -1 || resp_message.Error != "" {
		logger.Error("failed to get time on agent: %s", resp_message.Error)
		return "", fmt.Errorf(resp_message.Error)
	}

	return resp_message.Data.(string), nil
}

// 监控配置文件
func (a *Agent) ConfigfileInfo(ConMess global.ConfigMessage) error {
	msg := &protocol.Message{
		UUID: uuid.New().String(),
		Type: protocol.AgentConfig,
		Data: ConMess,
	}
	resp_message, err := a.sendMessage(msg, true, 0)
	if err != nil {
		logger.Error("failed to config on agent")
		return err
	}

	if resp_message.Status == -1 || resp_message.Error != "" {
		logger.Error("failed to config on agent: %s", resp_message.Error)
		return fmt.Errorf(resp_message.Error)
	}

	return nil
}

// 监控文件信息回传
func ConfigMessageInfo(Data interface{}) {
	p, ok := Data.(map[string]interface{})
	if ok {
		cf := dao.ConfigFile{
			MachineUUID: p["Machine_uuid"].(string),
			Content:     p["ConfigContent"].(string),
			Path:        p["ConfigName"].(string),
			UpdatedAt:   time.Time{},
		}
		err := dao.AddConfigFile(cf)
		if err != nil {
			logger.Error("配置文件添加失败" + err.Error())
		}
	}
}
