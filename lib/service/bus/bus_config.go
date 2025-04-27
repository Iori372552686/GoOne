package bus

// bus mq client config struct
type Config struct {
	LookupAddrs []string `json:"lookup_addrs" yaml:"lookup_addrs"`
	IPAddr      string   `json:"ip_addr" yaml:"ip_addr"`
	Port        int      `json:"port" yaml:"port"`
	Topics      string   `json:"topics" yaml:"topics"`
	ChanName    string   `json:"chan_name" yaml:"chan_name"`
	Concurrency int      `json:"concurrency" yaml:"concurrency"`
}
