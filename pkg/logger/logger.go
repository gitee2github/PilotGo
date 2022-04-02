/******************************************************************************
 * Copyright (c) KylinSoft Co., Ltd.2021-2022. All rights reserved.
 * PilotGo is licensed under the Mulan PSL v2.
 * You can use this software accodring to the terms and conditions of the Mulan PSL v2.
 * You may obtain a copy of Mulan PSL v2 at:
 *     http://license.coscl.org.cn/MulanPSL2
 * THIS SOFTWARE IS PROVIDED ON AN 'AS IS' BASIS, WITHOUT WARRANTIES OF ANY KIND,
 * EITHER EXPRESS OR IMPLIED, INCLUDING BUT NOT LIMITED TO NON-INFRINGEMENT, MERCHANTABILITY OR FIT FOR A PARTICULAR PURPOSE.
 * See the Mulan PSL v2 for more details.
 * Author: yangzhao1
 * Date: 2022-03-01 09:59:30
 * LastEditTime: 2022-04-02 15:30:32
 * Description: provide agent log manager of pilotgo
 ******************************************************************************/
package logger

import (
	"errors"
	"os"

	rotatelogs "github.com/lestrrat-go/file-rotatelogs"
	"github.com/sirupsen/logrus"
	conf "openeluer.org/PilotGo/PilotGo/pkg/config"
)

var logName string = "pilotgo"

func setLogDriver(logopts *conf.LogOpts) error {
	if logopts == nil {
		return errors.New("logopts is nil")
	}

	switch logopts.Driver {
	case "stdout":
		logrus.SetOutput(os.Stdout)
	case "file":
		writer, err := rotatelogs.New(
			logopts.Path+"/"+logName,
			rotatelogs.WithRotationCount(uint(logopts.MaxFile)),
			rotatelogs.WithRotationSize(int64(logopts.MaxSize)),
		)
		if err != nil {
			return err
		}
		logrus.SetOutput(writer)
	default:
		logrus.SetOutput(os.Stdout)
		logrus.Warn("!!! invalid log output, use stdout !!!")
	}
	return nil
}

func setLogLevel(logopts *conf.LogOpts) error {
	switch logopts.Level {
	case "trace":
		logrus.SetLevel(logrus.TraceLevel)
	case "debug":
		logrus.SetLevel(logrus.DebugLevel)
	case "info":
		logrus.SetLevel(logrus.InfoLevel)
	case "warn":
		logrus.SetLevel(logrus.WarnLevel)
	case "error":
		logrus.SetLevel(logrus.ErrorLevel)
	case "fatal":
		logrus.SetLevel(logrus.FatalLevel)
	default:
		return errors.New("invalid log level")
	}
	return nil
}
func Init(conf *conf.Configure) error {
	setLogLevel(&(conf.Logopts))
	err := setLogDriver(&(conf.Logopts))
	if err != nil {
		return err
	}
	logrus.Debug("log init")

	return nil
}

func Trace(format string, args ...interface{}) {
	logrus.Tracef(format, args...)
}

func Debug(format string, args ...interface{}) {
	logrus.Debugf(format, args...)
}

func Info(format string, args ...interface{}) {
	logrus.Infof(format, args...)
}

func Warn(format string, args ...interface{}) {
	logrus.Warnf(format, args...)
}

func Error(format string, args ...interface{}) {
	logrus.Errorf(format, args...)
}

func Fatal(format string, args ...interface{}) {
	logrus.Fatalf(format, args...)
}
