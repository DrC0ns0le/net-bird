package bird

import (
	"bufio"
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/DrC0ns0le/net-perf/logging"
)

const (
	birdSocket  = "/run/bird/bird.ctl"
	bird6Socket = "/run/bird/bird6.ctl" // for IPv6 Socket
	timeout     = 10 * time.Second
)

func Begin(mode string) (net.Conn, *bufio.Reader, *bufio.Writer, error) {
	conn, err := net.DialTimeout("unix", func() string {
		if mode == "v6" {
			return bird6Socket
		}
		return birdSocket
	}(), timeout)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("error connecting to BIRD socket: %v", err)
	}

	conn.SetDeadline(time.Now().Add(timeout))

	reader := bufio.NewReader(conn)
	writer := bufio.NewWriter(conn)

	// Read the welcome message
	welcome, err := reader.ReadString('\n')
	if err != nil {
		return nil, nil, nil, fmt.Errorf("error reading welcome message: %v", err)
	}
	logging.Debug("Welcome message: ", strings.TrimPrefix(welcome, "0001 "))

	return conn, reader, writer, nil
}
