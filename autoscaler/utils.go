package autoscaler

import (
	"fmt"
	"net"
	"time"
)

var PING_RETIRES = 3
var PING_TIMEOUT = 1 * time.Second

func ping(addrPort string) error {
	for i := 0; i < PING_RETIRES; i++ {
		timeout := time.Duration(PING_TIMEOUT)
		conn, err := net.DialTimeout("tcp", addrPort, timeout)
		defer conn.Close()
		if err == nil {
			return nil
		}
	}
	return fmt.Errorf("Remote %s didn't respond to ping", addrPort)
}
