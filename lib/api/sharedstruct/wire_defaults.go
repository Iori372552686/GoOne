package sharedstruct

import "time"

const (
	// ClientHeartBeatInterval is the expected interval between client heartbeats.
	ClientHeartBeatInterval = 5 * time.Second
	// ClientExpiryThreshold is the default window for client inactivity.
	ClientExpiryThreshold = 6 * ClientHeartBeatInterval
)

// ServerTypeInCmd extracts the destination service type from command bits 23–16.
func ServerTypeInCmd(cmd uint32) uint32 {
	return cmd >> 16 & 0xff
}
