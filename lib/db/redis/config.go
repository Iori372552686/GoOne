package redis

//redis config struct
type Config struct {
	InstanceID  int    `json:"InstanceId"`
	IP          string `json:"Ip"`
	Port        int    `json:"Port"`
	Password    string `json:"Password"`
	IsCluster   bool   `json:"IsCluster"`
	DbIndex     int    `json:"DbIndex"`
	Description string `json:"Description"`
}
