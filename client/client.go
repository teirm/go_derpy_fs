// bare bones client implementation
package main

import (
	"flag"
	"log"
	"net"
	"os"
	"strconv"
	"strings"
)

const (
	defaultAddress string = "127.0.0.1"
	defaultPort    string = "0"
)

type ClientConfig struct {
	ip          *string
	port        *string
	op          *string
	file        *string
	interactive *bool
}

type ClientState struct {
	conn net.Conn
}

// Create a header
func createHeader(op string, account string, fileName string, size uint64) string {
	sizeStr := strconv.FormatUint(size, 64)
	s := []string{op, account, fileName, sizeStr}
	return strings.Join(s, ":")
}

// create conection to server
func connect(ip string, port string) (net.Conn, error) {
	address := ip + ":" + port
	return net.Dial("tcp", address)
}

// initialize and start client
func startClient(config ClientConfig) error {
	var client ClientState
	var err error

	client.conn, err = connect(*config.ip, *config.port)
	if err != nil {
		log.Printf("unable to connect to server: %v\n", err)
		return err
	}

	if config.interactive == false {
		// handle non interactive sessions
	} else {
		// start an interactive session
		// with channels
	}

	return nil
}

func main() {
	var config ClientConfig
	config.ip = flag.String("address", defaultAddress, "address to connect to")
	config.port = flag.String("port", defaultPort, "port to connect to")
	config.op = flag.String("op", "NOOP", "operation to perform")
	config.file = flag.String("file-name", "", "file to read or write into")
	config.interactive = flag.Bool("interactive", false, "start an interactice session")

	flag.Parse()

	err := startClient(config)
	if err != nil {
		os.Exit(1)
	} else {
		os.Exit(0)
	}
}
