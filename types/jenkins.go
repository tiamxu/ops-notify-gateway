package types

type JenkinsNotifyReq struct {
	Channel     string   `json:"channel"`
	Status      string   `json:"status"`
	JobName     string   `json:"jobName"`
	BuildNumber string   `json:"buildNumber"`
	BuildURL    string   `json:"buildUrl"`
	Services    []string `json:"services"`
	Branch      string   `json:"branch"`
	DeployEnv   string   `json:"deployEnv"`
	PackageType string   `json:"packageType"`
	Deploy      bool     `json:"deploy"`
	DeployType  string   `json:"deployType"`
	TriggerUser string   `json:"triggerUser"`
}
