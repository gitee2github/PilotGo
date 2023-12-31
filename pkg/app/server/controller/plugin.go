package controller

import (
	"net/http"
	"net/http/httputil"
	"net/url"

	"github.com/gin-gonic/gin"
	"openeuler.org/PilotGo/PilotGo/pkg/app/server/service/plugin"
	"openeuler.org/PilotGo/PilotGo/pkg/logger"
	"openeuler.org/PilotGo/PilotGo/pkg/utils/response"
)

// 查询插件清单
func GetPluginsHandler(c *gin.Context) {
	plugins := plugin.GetPlugins()

	logger.Info("find %d plugins", len(plugins))
	response.Success(c, plugins, "插件查询成功")
}

// 添加插件
func AddPluginHandler(c *gin.Context) {
	param := struct {
		Url string `json:"url"`
	}{}

	if err := c.BindJSON(&param); err != nil {
		response.Fail(c, nil, "参数错误")
		return
	}

	if err := plugin.AddPlugin(param.Url); err != nil {
		response.Fail(c, nil, "add plugin failed:"+err.Error())
		return
	}

	response.Success(c, nil, "插件添加成功")
}

// 停用/启动插件
func TogglePluginHandler(c *gin.Context) {
	param := struct {
		UUID   string `json:"uuid"`
		Enable int    `json:"enable"`
	}{}

	if err := c.BindJSON(&param); err != nil {
		response.Fail(c, nil, "参数错误")
		return
	}

	logger.Info("toggle plugin:%s to enable %d", param.UUID, param.Enable)
	plugin.TogglePlugin(param.UUID, param.Enable)
	response.Success(c, nil, "插件信息更新成功")
}

// 卸载插件
func UnloadPluginHandler(c *gin.Context) {
	uuid := c.Param("uuid")
	if uuid == "undefined" {
		response.Fail(c, nil, "参数错误")
		return
	}

	logger.Info("unload plugin:%s", uuid)
	plugin.DeletePlugin(uuid)

	response.Success(c, nil, "插件信息更新成功")
}

func PluginGatewayHandler(c *gin.Context) {
	// TODO
	name := c.Param("plugin_name")
	p := plugin.GetPlugin(name)
	if p == nil {
		c.String(http.StatusNotFound, "plugin not found")
		return
	}

	target, err := url.Parse(p.Url)
	if err != nil {
		// return 404
		c.String(http.StatusNotFound, "parse plugin url error: "+err.Error())
		return
	}
	proxy := httputil.NewSingleHostReverseProxy(target)
	proxy.ServeHTTP(c.Writer, c.Request)
}
