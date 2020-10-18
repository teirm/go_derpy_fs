// bare bones client implementation
package main

import (
	"container/list"
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"path"
	"sync"

	"github.com/teirm/go_ftp/common"
)

const (
	defaultAddress string = "127.0.0.1"
	defaultPort    string = "0"
)

type ClientConfig struct {
	ip      string
	port    string
	account string
	op      string
	file    string
}

type ClientState struct {
	conn      net.Conn
	diskWrite chan common.ResponseData
	diskRead  chan common.ClientData
	send      chan common.ClientData
	read      chan net.Conn
	wg        sync.WaitGroup
}

// create conection to server
func connect(ip string, port string) (net.Conn, error) {
	address := ip + ":" + port
	return net.Dial("tcp", address)
}

// performOperation
func performOperation(config ClientConfig, client *ClientState) error {

	account := config.account
	fileName := config.file
	switch config.op {
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
func doCreate(account string, client *ClientState) {
	header := common.Header{"CREATE", account, "", 0}
	client.wg.Add(2)
	client.send <- common.ClientData{header, nil, client.conn}
	client.read <- client.conn
}

// do a read operation
func doRead(account string, fileName string, client *ClientState) {
	header := common.Header{"READ", account, fileName, 0}
	client.wg.Add(2)
	client.send <- common.ClientData{header, nil, client.conn}
	client.read <- client.conn
}

// do a write operation
func doWrite(account string, fileName string, client *ClientState) {
	header := common.Header{"WRITE", account, fileName, 0}
	client.wg.Add(1)
	client.diskRead <- common.ClientData{header, nil, client.conn}
}

// do a delete operation
func doDelete(account string, fileName string, client *ClientState) {
	header := common.Header{"DELETE", account, fileName, 0}
	client.wg.Add(2)
	client.send <- common.ClientData{header, nil, client.conn}
	client.read <- client.conn
}

// do a list operation
func doList(account string, client *ClientState) {
	header := common.Header{"LIST", account, "", 0}
	client.wg.Add(2)
	client.send <- common.ClientData{header, nil, client.conn}
	client.read <- client.conn
}

// Basic sanity checking on configuration
func validateConfig(config *ClientConfig) error {
	if config.account == "" {
		return fmt.Errorf("invalid account name: %s", config.account)
	}

	if err := common.CheckOperation(config.op); err != nil {
		return err
	}

	return nil
}

// Send a message to the file server
func sendMessage(data common.ClientData) error {
	serializedHeader := common.SerializeHeader(data.Header)
	return common.SendMessage(serializedHeader, data.DataList, data.Conn)
}

// Perform disk IO
func doDiskRead(data *common.ClientData) error {
	flags := os.O_RDONLY
	perms := os.FileMode(0644)
	data.DataList = list.New()
	size, err := common.ReadFile(data.Header.FileName, flags, perms, data.DataList)
	if err != nil {
		return err
	}
	data.Header.FileName = path.Base(data.Header.FileName)
	data.Header.Size = size
	return nil
}

// Perform disk IO
func doDiskWrite(data *common.ResponseData) error {
	flags := os.O_APPEND | os.O_WRONLY | os.O_CREATE
	perms := os.FileMode(0644)
	fileName := data.Header.FileName
	err := common.WriteFile(fileName, flags, perms, data.DataList)
	return err
}

// Read responses from the server
func readResponse(conn net.Conn) (common.ResponseData, error) {
	responseHeader, err := common.ReadHeader(conn)
	if err != nil {
		common.DebugLog("Error reading response header: %v\n", err)
		return common.ResponseData{}, err
	}
	var response common.ResponseData
	response.Header = responseHeader
	response.Conn = conn
	response.DataList = list.New()

	readSize := responseHeader.Size
	if err := common.ReadMessage(response.DataList, readSize, conn); err != nil {
		return common.ResponseData{}, fmt.Errorf("error reading response: %v", err)
	}

	return response, nil
}

// Handle responses from the server
func handleResponse(response common.ResponseData, cli *ClientState) {
	header := response.Header
	common.DebugLog("response header: %v", header)
	switch header.Operation {
	case "READ":
		cli.diskWrite <- response
	case "LIST":
		dataList := response.DataList
		for iter := dataList.Front(); iter != nil; iter = iter.Next() {
			data := iter.Value.(common.Data)
			log.Printf("%s", string(data.Buffer))
		}
	default:
		log.Printf("%s\n", header.Info)
	}
}

// initialize and start client
func startClient(ip string, port string) (*ClientState, error) {
	var client ClientState
	var err error

	// TODO: connecting so early might be problematic
	// if disk is slow. Maybe connect closer to when
	// doing network IO
	client.conn, err = connect(ip, port)
	if err != nil {
		log.Printf("unable to connect to server: %v\n", err)
		return nil, err
	}

	// default to non-interactive worker count
	netWorkers := 1
	diskReaders := 1
	diskWriters := 1
	respWorkers := 1

	client.diskWrite = make(chan common.ResponseData)
	client.diskRead = make(chan common.ClientData)
	client.send = make(chan common.ClientData)
	client.read = make(chan net.Conn)

	for i := 0; i < netWorkers; i++ {
		go func(cli *ClientState) {
			for data := range cli.send {
				common.DebugLog("%v\n", data)
				err := sendMessage(data)
				if err != nil {
					log.Printf("unable to send message: %v\n", err)
				}
				cli.wg.Done()
			}
		}(&client)
	}

	for i := 0; i < diskReaders; i++ {
		go func(cli *ClientState) {
			for data := range cli.diskRead {
				err := doDiskRead(&data)
				if err != nil {
					log.Printf("unable to perform disk io: %v\n", err)
				}
				cli.wg.Add(2)
				cli.send <- data
				cli.read <- data.Conn
				cli.wg.Done()
			}
		}(&client)
	}

	for i := 0; i < diskWriters; i++ {
		go func(cli *ClientState) {
			for data := range cli.diskWrite {
				err := doDiskWrite(&data)
				if err != nil {
					log.Printf("unable to perform disk write: %v\n", err)
				}
				cli.wg.Done()
			}
		}(&client)
	}

	for i := 0; i < respWorkers; i++ {
		go func(cli *ClientState) {
			common.DebugLog("In response loop\n")
			for data := range cli.read {
				common.DebugLog("Received data to read\n")
				response, err := readResponse(data)
				if err != nil {
					log.Printf("unable to read response: %v\n", err)
				} else {
					handleResponse(response, cli)
				}
				cli.wg.Done()
			}
		}(&client)
	}

	return &client, nil
}

func main() {
	var config ClientConfig
	flag.StringVar(&config.ip, "address", defaultAddress, "address to connect to")
	flag.StringVar(&config.port, "port", defaultPort, "port to connect to")
	flag.StringVar(&config.account, "account", "", "account to access")
	flag.StringVar(&config.op, "op", "NOOP", "operation to perform")
	flag.StringVar(&config.file, "file-name", "", "file to read or write into")
	common.AddCommonFlags()

	flag.Parse()

	cli, err := startClient(config.ip, config.port)
	if err != nil {
		os.Exit(1)
	}

	err = performOperation(config, cli)

	cli.wg.Wait()
	if err != nil {
		os.Exit(1)
	} else {
		os.Exit(0)
	}
}
