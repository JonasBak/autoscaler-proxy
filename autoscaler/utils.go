package autoscaler

import (
	"fmt"
	"net"
	"time"
)

func ping(retries int, timeout int, wait int, addrPort string) error {
	return pingConn(retries, wait, func() (net.Conn, error) {
		timeout := time.Duration(time.Duration(timeout) * time.Second)
		return net.DialTimeout("tcp", addrPort, timeout)
	})
}

func pingConn(retries int, wait int, getConn func() (net.Conn, error)) error {
	for i := 0; i < retries; i++ {
		conn, err := getConn()
		if err == nil {
			conn.Close()
			return nil
		}
		time.Sleep(time.Duration(wait) * time.Second)
	}
	return fmt.Errorf("Remote didn't respond to ping")
}
