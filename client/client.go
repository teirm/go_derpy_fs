// bare bones client implementation
package main

import (
	"container/list"
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"strconv"
	"strings"

	"github.com/teirm/go_ftp/common"
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
	disk chan common.ClientData
	send chan common.ClientData
	read chan net.Conn
}

// Create a header
func serializeHeader(header common.Header) []byte {
	sizeStr := strconv.FormatUint(header.Size, 10)
	s := strings.Join([]string{header.Operation, header.Account, header.FileName, sizeStr}, ":")
	return []byte(s + "\n")
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
		doCreate(account, client)
	case "READ":
		doRead(account, fileName, client)
	case "WRITE":
		doWrite(account, fileName, client)
	case "DELETE":
		doDelete(account, fileName, client)
	case "LIST":
		doList(account, client)
	}
	return nil
}

// do a create operation for a new account
func doCreate(account string, client ClientState) {
	header := common.Header{"CREATE", account, "", 0}
	client.send <- common.ClientData{header, nil, client.conn}
}

// do a read operation
func doRead(account string, fileName string, client ClientState) {
	header := common.Header{"READ", account, fileName, 0}
	client.send <- common.ClientData{header, nil, client.conn}
}

// do a write operation
func doWrite(account string, fileName string, client ClientState) {
	header := common.Header{"WRITE", account, fileName, 0}
	client.disk <- common.ClientData{header, nil, client.conn}
}

// do a delete operation
func doDelete(account string, fileName string, client ClientState) {
	header := common.Header{"DELETE", account, fileName, 0}
	client.send <- common.ClientData{header, nil, client.conn}
}

// do a list operation
func doList(account string, client ClientState) {
	header := common.Header{"LIST", account, "", 0}
	client.send <- common.ClientData{header, nil, client.conn}
}

// Basic sanity checking on configuration
func validateConfig(config ClientConfig) error {
	if *config.account == "" {
		return fmt.Errorf("invalid account name: %s", *config.account)
	}

	if err := common.CheckOperation(*config.op); err != nil {
		return err
	}

	return nil
}

// Send a message to the file server
func sendMessage(data common.ClientData) error {
	serializedHeader := serializeHeader(data.Header)
	return common.SendMessage(serializedHeader, data.DataList, data.Conn)
}

// Perform disk IO
func doDiskIO(data *common.ClientData) error {
	return nil
}

// Read responses from the server
func readResponse(conn net.Conn) error {
	dataList := list.New()
	return common.ReadMessage(dataList, conn)
}

// initialize and start client
func startClient(ip string, port string, interactive bool) (ClientState, error) {
	var client ClientState
	var err error

	// TODO: connecting so early might be problematic
	// if disk is slow. Maybe connect closer to when
	// doing network IO
	client.conn, err = connect(ip, port)
	if err != nil {
		log.Printf("unable to connect to server: %v\n", err)
		return ClientState{}, err
	}

	// default to non-interactive worker count
	netWorkers := 1
	diskWorkers := 1
	respWorkers := 1
	if interactive == true {
		netWorkers = 3
		diskWorkers = 3
		respWorkers = 3
	}

	client.disk = make(chan common.ClientData)
	client.send = make(chan common.ClientData)
	client.read = make(chan net.Conn)

	for i := 0; i < netWorkers; i++ {
		go func(cli ClientState) {
			for data := range cli.send {
				err := sendMessage(data)
				if err != nil {
					log.Printf("unable to send message: %v\n", err)
				}
			}
		}(client)
	}

	for i := 0; i < diskWorkers; i++ {
		go func(cli ClientState) {
			for data := range cli.disk {
				err := doDiskIO(&data)
				if err != nil {
					log.Printf("unable to perform disk io: %v\n", err)
				}
			}
		}(client)
	}

	for i := 0; i < respWorkers; i++ {
		go func(cli ClientState) {
			for data := range cli.read {
				err := readResponse(data)
				if err != nil {
					log.Printf("unable to read reasponse: %v\n", err)
				}
			}
		}(client)
	}

	return client, nil
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

	cli, err := startClient(*config.ip, *config.port, *config.interactive)
	if err != nil {
		os.Exit(1)
	}

	err = performOperation(config, cli)
	if err != nil {
		os.Exit(1)
	} else {
		os.Exit(0)
	}
}
