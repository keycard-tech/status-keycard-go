package api

import "C"
import (
	"github.com/status-im/status-keycard-go/internal"
)

func Stop(request string) interface{} {
	GlobalKeycardService.keycardContext.Stop()
	return buildResponse(nil, nil)
}

func SelectApplet(request string) interface{} {
	info, err := GlobalKeycardService.keycardContext.SelectApplet()
	appInfo := internal.ToAppInfo(info)
	return buildResponse(appInfo, err)
}
