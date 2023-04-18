package baseos

import (
	"bufio"
	"fmt"
	"strings"

	"openeuler.org/PilotGo/PilotGo/pkg/logger"
	"openeuler.org/PilotGo/PilotGo/pkg/utils"
	"openeuler.org/PilotGo/PilotGo/pkg/utils/os/common"
)

const (
	ServiceStart   = 0
	ServiceStop    = 1
	ServiceRestart = 2
)

// 获取服务列表
func (b *BaseOS) GetServiceList() ([]common.ListService, error) {
	list := make([]common.ListService, 0)
	result1, err := utils.RunCommand("systemctl list-units --all|grep 'loaded[ ]*active' | awk 'NR>2{print $1\" \" $2\" \" $3\" \" $4}'")
	if err != nil {
		logger.Error("failed to execute the command to get the list of services: %s", err)
		return []common.ListService{}, err
	}
	result2, err := utils.RunCommand("systemctl list-units --all|grep 'not-found' | awk 'NR>2{print $2\" \" $3\" \" $4\" \" $5}'")
	if err != nil {
		logger.Error("the command to get the list of services has failed to run: %s", err)
		return []common.ListService{}, err
	}
	result := result1 + result2
	reader := strings.NewReader(result)
	scanner := bufio.NewScanner(reader)
	for {

		if !scanner.Scan() {
			break
		}
		line := scanner.Text()
		line = strings.TrimSpace(line)
		str := strings.Fields(line)
		tmp := common.ListService{}
		tmp.Name = str[0]
		tmp.LOAD = str[1]
		tmp.Active = str[2]
		tmp.SUB = str[3]
		list = append(list, tmp)
	}
	return list, nil
}

//operate 0,1,2分别代表开启，关闭，重启
// TODO: 软件包在未安装情况下，'systemctl is-active'返回的结果和软件包已安装且服务未运行时的结果一致
func (b *BaseOS) GetServiceStatus(service string) (string, error) {
	var build strings.Builder
	build.WriteString("systemctl is-active ")
	build.WriteString(service)
	command := build.String()
	tmp, err := utils.RunCommand(command)
	output := strings.Trim(tmp, "\n")
	switch output {
	case "active", "inactive":
		return output, nil
	default:
		return output, err
	}
}
func verifyStatus(output string, operate int) bool {
	var judge bool
	if output == "inactive" {
		switch operate {
		case 0:
			judge = false
		case 1:
			judge = true
		case 2:
			judge = false
		}
	} else {
		switch operate {
		case 0:
			judge = true
		case 1:
			judge = false
		case 2:
			judge = true
		}
	}
	return judge
}
func (b *BaseOS) StartService(service string) error {
	var build strings.Builder
	build.WriteString("systemctl start ")
	build.WriteString(service)
	command := build.String()
	result, err := utils.RunCommand(command)
	if err != nil || len(result) != 0 {
		logger.Error("failed to execute the command to start the service: %s", err)
		return fmt.Errorf(" failed to start the %s service: %s", service, err)
	}
	output, err := b.GetServiceStatus(service)
	if err != nil {
		logger.Error("failed to retrieve the status of the service")
		return fmt.Errorf("failed to start the %s service: %s", service, err)
	}
	if !verifyStatus(output, ServiceStart) {
		logger.Error("the command to start the service has produced an invalid result!")
		return fmt.Errorf("failed to start the %s service: %s", service, err)
	}
	return nil
}
func (b *BaseOS) StopService(service string) error {
	var build strings.Builder
	build.WriteString("systemctl stop ")
	build.WriteString(service)
	command := build.String()
	result, err := utils.RunCommand(command)
	if err != nil || len(result) != 0 {
		logger.Error("failed to execute the command to stop the service: %s", err)
		return fmt.Errorf("failed to stop the %s service: %s", service, err)
	}
	output, err := b.GetServiceStatus(service)
	if err != nil {
		logger.Error("failed to get the status of the service")
		return fmt.Errorf("failed to stop the %s service: %s", service, err)
	}
	if !verifyStatus(output, ServiceStop) {
		logger.Error("the command to stop the service has produced an invalid result!")
		return fmt.Errorf("failed to stop the %s service: %s", service, err)
	}
	return nil
}
func (b *BaseOS) RestartService(service string) error {
	var build strings.Builder
	build.WriteString("systemctl restart ")
	build.WriteString(service)
	command := build.String()
	result, err := utils.RunCommand(command)
	if err != nil || len(result) != 0 {
		logger.Error("failed to execute the command to restart the service: %s", err)
		return fmt.Errorf("failed to restart the %s service: %s", service, err)
	}
	output, err := b.GetServiceStatus(service)
	if err != nil {
		logger.Error("failed to get the status of the service")
		return fmt.Errorf("failed to restart the %s service: %s", service, err)
	}
	if !verifyStatus(output, ServiceRestart) {
		logger.Error("failed to execute the command to restart the service!")
		return fmt.Errorf("failed to restart the %s service: %s", service, err)
	}
	return nil
}
