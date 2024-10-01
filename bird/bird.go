package bird

import (
	"bufio"
	"fmt"
	"net"
	"os"
	"strings"
	"time"
)

const (
	birdSocket = "/run/bird/bird.ctl"
	timeout    = 10 * time.Second
)

func Begin() (net.Conn, *bufio.Reader, *bufio.Writer, error) {
	conn, err := net.DialTimeout("unix", birdSocket, timeout)
	if err != nil {
		fmt.Printf("Error connecting to BIRD socket: %v\n", err)
		os.Exit(1)
	}

	conn.SetDeadline(time.Now().Add(timeout))

	reader := bufio.NewReader(conn)
	writer := bufio.NewWriter(conn)

	// Read the welcome message
	welcome, err := reader.ReadString('\n')
	if err != nil {
		fmt.Printf("Error reading welcome message: %v\n", err)
		os.Exit(1)
	}
	fmt.Print("Welcome message: ", strings.TrimPrefix(welcome, "0001 "))

	return conn, reader, writer, nil
}
