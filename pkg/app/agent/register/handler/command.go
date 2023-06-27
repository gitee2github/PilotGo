package handler

import (
	"encoding/base64"
	"path"
	"strings"

	"github.com/google/uuid"

	"openeuler.org/PilotGo/PilotGo/pkg/app/agent/network"
	"openeuler.org/PilotGo/PilotGo/pkg/logger"
	"openeuler.org/PilotGo/PilotGo/pkg/utils"
	"openeuler.org/PilotGo/PilotGo/pkg/utils/message/protocol"
)

func RunCommandHandler(c *network.SocketClient, msg *protocol.Message) error {
	logger.Debug("process run command:%s", msg.String())

	d := &struct {
		Command string
	}{}

	err := msg.BindData(d)
	if err != nil {
		resp_msg := &protocol.Message{
			UUID:   msg.UUID,
			Type:   msg.Type,
			Status: -1,
			Error:  "parse data error:" + err.Error(),
		}
		return c.Send(resp_msg)
	}

	content, err := base64.StdEncoding.DecodeString(d.Command)
	if err != nil {
		resp_msg := &protocol.Message{
			UUID:   msg.UUID,
			Type:   msg.Type,
			Status: -1,
			Error:  "run command error:" + err.Error(),
		}
		return c.Send(resp_msg)
	}

	retCode, stdout, stderr, err := utils.RunCommand(string(content))
	if err != nil {
		resp_msg := &protocol.Message{
			UUID:   msg.UUID,
			Type:   msg.Type,
			Status: -1,
			Error:  "run command error:" + err.Error(),
		}
		return c.Send(resp_msg)
	}

	resp_msg := &protocol.Message{
		UUID:   msg.UUID,
		Type:   msg.Type,
		Status: 0,
		Data: &utils.CmdResult{
			RetCode: retCode,
			Stdout:  stdout,
			Stderr:  stderr,
		},
	}
	return c.Send(resp_msg)
}

func RunScriptHandler(c *network.SocketClient, msg *protocol.Message) error {
	errorInfo := ""
	resp_msg := &protocol.Message{
		UUID:   msg.UUID,
		Type:   msg.Type,
		Status: 0,
	}

	var result *utils.CmdResult
	var scriptPath string
	var err error
	var decoded_script []byte

	d := &struct {
		Script string
	}{}

	f := func(s string) (string, error) {
		workDir := "/opt/PilotGo/agent/"

		fileName := uuid.New().String()
		filePath := path.Join(workDir, fileName+".sh")

		err = utils.FileSaveString(filePath, s)
		if err != nil {
			return "", err
		}

		return filePath, nil
	}

	err = msg.BindData(d)
	if err != nil {
		errorInfo = "parse data error:" + err.Error()
		logger.Error(errorInfo)
		goto ERROR
	}

	decoded_script, err = base64.StdEncoding.DecodeString(d.Script)
	if err != nil {
		errorInfo = "Err decoding base64: " + err.Error()
		logger.Error(errorInfo)
		goto ERROR
	}
	logger.Debug("process run script command: %s", string(decoded_script))

	scriptPath, err = f(strings.Replace(string(decoded_script), "\r", "", -1))
	if err != nil {
		errorInfo = "run command error:" + err.Error()
		logger.Error(errorInfo)
		goto ERROR
	}

	result, err = utils.RunScript(scriptPath)
	if err != nil {
		errorInfo = "run command error:" + err.Error()
		logger.Error(errorInfo)
		goto ERROR
	}

	resp_msg.Status = 0
	resp_msg.Data = result
	return c.Send(resp_msg)

ERROR:
	resp_msg.Status = -1
	resp_msg.Error = errorInfo
	return c.Send(resp_msg)
}
