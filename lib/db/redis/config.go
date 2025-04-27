package redis

// db config struct
type Config struct {
	InstanceID  int    `json:"InstanceId" yaml:"InstanceId"`
	IP          string `json:"Ip" yaml:"Ip"`
	Port        int    `json:"Port" yaml:"Port"`
	Password    string `json:"Password" yaml:"Password"`
	IsCluster   bool   `json:"IsCluster" yaml:"IsCluster"`
	DbIndex     int    `json:"DbIndex" yaml:"DbIndex"`
	Description string `json:"Description" yaml:"Description"`
}
