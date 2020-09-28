// bare bones client implementation
package main

import (
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"strconv"
	"strings"

	"github.com/teirm/go_derpy_fs/common"
)

const (
	defaultAddress string = "127.0.0.1"
	defaultPort    string = "0"
)

type ClientConfig struct {
	ip          *string
	port        *string
	account     *string
	op          *string
	file        *string
	interactive *bool
}

type ClientState struct {
	conn net.Conn
	ioCh chan string
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

// performOperation
func performOperation(config ClientConfig, client ClientState) error {

	account := *config.account
	fileName := *config.file
	switch *config.op {
	case "CREATE":
		return doCreate(account, client)
	case "READ":
		return doRead(account, fileName, client)
	case "WRITE":
		return doWrite(account, fileName, client)
	case "DELETE":
		return doDelete(account, fileName, client)
	case "LIST":
		return doList(account, client)
	}
	return nil
}

// do a create operation for a new account
func doCreate(account string, client ClientState) error {
	return nil
}

// do a read operation
func doRead(account string, fileName string, client ClientState) error {
	return nil
}

// do a write operation
func doWrite(account string, fileName string, client ClientState) error {
	return nil
}

// do a delete operation
func doDelete(account string, fileName string, client ClientState) error {
	return nil
}

// do a list operation
func doList(account string, client ClientState) error {
	return nil
}

// handle a non-interactive session
func nonInteractiveSession(config ClientConfig, client ClientState) error {
	if *config.account == "" {
		return fmt.Errorf("invalid account name: %s", *config.account)
	}

	if err := common.CheckOperation(*config.op); err != nil {
		return err
	}

	return nil
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

	if *config.interactive == false {
		// there is no reason for the non-interactive
		// session to not setup multiple channels and
		// pipline work just like an interactive session would
		// the only difference is that it would block.
		err = nonInteractiveSession(config, client)
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
	config.account = flag.String("account", "", "account to access")
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
