package types

type Response struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data"`
}

func Success(data interface{}) Response {
	return Response{Code: 200, Message: "操作成功", Data: data}
}

func Error(code int, message string) Response {
	return Response{Code: code, Message: message, Data: nil}
}
