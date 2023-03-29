package autoscaler

import (
	"fmt"
	"net"
	"time"
)

func ping(retries int, timeout int, wait int, addrPort string) error {
	for i := 0; i < retries; i++ {
		timeout := time.Duration(time.Duration(timeout) * time.Second)
		conn, err := net.DialTimeout("tcp", addrPort, timeout)
		if err == nil {
			conn.Close()
			return nil
		}
		time.Sleep(time.Duration(wait) * time.Second)
	}
	return fmt.Errorf("Remote %s didn't respond to ping", addrPort)
}
