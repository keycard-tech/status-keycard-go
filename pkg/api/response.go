package api

type response struct {
	Error string      `json:"error"`
	Data  interface{} `json:"data"`
}

func buildResponse(data interface{}, err error) response {
	resp := response{
		Data: data,
	}
	if err != nil {
		resp.Error = err.Error()
	}
	return resp
	//output, err := json.Marshal(resp)
	//if err != nil {
	//	zap.L().Error("failed to marshal response", zap.Error(err))
	//}
	//return string(output)
}
