package ssdb

//ssdb config
type Config struct {
	InstanceID   int    `json:"InstanceId"`
	Key          string `json:"Key"`
	IP           string `json:"Ip"`
	Port         int    `json:"Port"`
	User         string `json:"User"`
	Password     string `json:"Password"`
	Description  string `json:"Description"`
	HealthSecond int    `json:"HealthSecond"`
	MaxPool      int    `json:"MaxPool"`
	AutoClose    bool   `json:"AutoClose"`
	MaxWaitSize  int    `json:"MaxWaitSize"`
}
