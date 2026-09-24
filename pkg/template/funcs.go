package template

import "time"

func FuncMap() map[string]interface{} {
	return map[string]interface{}{
		"GetCSTtime": GetCSTtime,
		"Now":        func() string { return time.Now().Format("2006-01-02 15:04:05") },
	}
}

func GetCSTtime(value string) string {
	if value == "" {
		return ""
	}
	parsed, err := time.Parse(time.RFC3339Nano, value)
	if err != nil {
		return value
	}
	loc, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		loc = time.FixedZone("CST", 8*3600)
	}
	return parsed.In(loc).Format("2006-01-02 15:04:05")
}
