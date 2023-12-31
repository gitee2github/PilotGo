//

package pluginapi

import (
	"github.com/gin-gonic/gin"
	"openeuler.org/PilotGo/PilotGo/pkg/app/server/service/eventbus"
	"openeuler.org/PilotGo/PilotGo/pkg/logger"
	"openeuler.org/PilotGo/PilotGo/pkg/utils/response"
)

func RegisterListenerHandler(c *gin.Context) {
	p := struct {
		Name string `json:"name"`
		URL  string `json:"url"`
	}{}
	if err := c.ShouldBindQuery(p); err != nil {
		response.Fail(c, gin.H{"status": false}, err.Error())
		return
	}

	eventbus.AddListener(&eventbus.Listener{
		Name: p.Name,
		URL:  p.URL,
	})

	logger.Info("")
}

func UnregisterListenerHandler(c *gin.Context) {
	p := struct {
		Name string `json:"name"`
		URL  string `json:"url"`
	}{}
	if err := c.ShouldBindQuery(p); err != nil {
		response.Fail(c, gin.H{"status": false}, err.Error())
		return
	}

	eventbus.RemoveListener(&eventbus.Listener{
		Name: p.Name,
		URL:  p.URL,
	})
}
