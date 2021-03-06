// Server for journal application
package main

import (
	"container/list"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"os"
	"path"

	"github.com/teirm/go_ftp/common"
)

const (
	defaultPort string = "0"

	handleConnWorkers int = 3
	ioWorkers         int = 3
	respWorkers       int = 3

	headerDelim string = ":"

	accountRoot  string      = "/tmp"
	defaultPerms os.FileMode = 0644
)

// Server instance containing channels and connections
type Server struct {
	listener net.Listener

	handleChan chan net.Conn
	ioChan     chan common.ClientData
	respChan   chan common.ResponseData
}

// handle a connection and read client data
// pass client data to io worker
// on error pass error to response worker
func handleConnection(connection net.Conn, svr Server) error {
	header, err := common.ReadHeader(connection)
	if err != nil {
		return fmt.Errorf("failed to parse header: %v", err)
	}

	var message common.ClientData
	message.Header = header
	message.Conn = connection
	message.DataList = list.New()

	readSize := message.Header.Size
	if err := common.ReadMessage(message.DataList, readSize, connection); err != nil {
		return fmt.Errorf("error processing input: %v", err)
	}

	svr.ioChan <- message
	return nil
}

// create a ResponseData
func createResponseData(op string, result string, fileName string, size uint64, dataList *list.List, conn net.Conn) common.ResponseData {
	header := common.Header{op, result, fileName, size}
	return common.ResponseData{header, dataList, conn}
}

// Perform the requested server side IO operation
// and produce an error or response for the client
func handleIO(data common.ClientData, svr Server) error {

	header := data.Header
	op := header.Operation

	var res common.ResponseData
	var err error
	switch op {
	case "CREATE":
		res, err = createAccount(data.Header.Info, data.Conn)
	case "WRITE":
		res, err = writeFile(data.Header.Info, data.Header.FileName, data.DataList, data.Conn)
	case "READ":
		res, err = readFile(data.Header.Info, data.Header.FileName, data.Conn)
	case "DELETE":
		res, err = deleteFile(data.Header.Info, data.Header.FileName, data.Conn)
	case "LIST":
		res, err = listFiles(data.Header.Info, data.Conn)
	default:
		return fmt.Errorf("Invalid operation: %s", op)
	}
	if err != nil {
		return err
	}

	svr.respChan <- res
	return nil
}

// Check if the given path exists or not
func checkExistence(path string) (bool, error) {
	_, err := os.Stat(path)
	if err == nil {
		return true, nil
	}

	if os.IsNotExist(err) == true {
		return false, nil
	}

	// an error occured with stat
	return false, err
}

// Create a new account for the given identity
//
// By definition, an account will just be a
// new directory
func createAccount(account string, conn net.Conn) (common.ResponseData, error) {
	accountPath := path.Join(accountRoot, account)
	exists, err := checkExistence(accountPath)
	if err != nil {
		return common.ResponseData{}, err
	}
	if exists == true {
		err := fmt.Errorf("%s already exists", account)
		return common.ResponseData{}, err
	}

	err = os.Mkdir(accountPath, os.FileMode(0744))
	if err != nil {
		return common.ResponseData{}, err
	}

	resp := fmt.Sprintf("account created %s", account)
	return createResponseData("CREATE", resp, "", 0, nil, conn), nil
}

// Write a file under the given account
//
// Write will fail if the file exists already
func writeFile(account string, fileName string, dataList *list.List, conn net.Conn) (common.ResponseData, error) {
	filePath := path.Join(accountRoot, account, fileName)

	openFlags := os.O_APPEND | os.O_CREATE | os.O_WRONLY
	if err := common.WriteFile(filePath, openFlags, defaultPerms, dataList); err != nil {
		return common.ResponseData{}, err
	}

	resp := fmt.Sprintf("wrote file %s", fileName)
	return createResponseData("WRITE", resp, "", 0, nil, conn), nil
}

// Read a file under the given account
//
// Read will fail if the file does not exist
func readFile(account string, fileName string, conn net.Conn) (common.ResponseData, error) {
	filePath := path.Join(accountRoot, account, fileName)

	dataList := list.New()
	size, err := common.ReadFile(filePath, os.O_RDONLY, defaultPerms, dataList)
	if err != nil {
		return common.ResponseData{}, err
	}

	resp := fmt.Sprintf("read file %s", fileName)
	return createResponseData("READ", resp, fileName, size, dataList, conn), nil
}

// Delete a file under the given account
//
// Delete will fail if the file does not exist
func deleteFile(account string, fileName string, conn net.Conn) (common.ResponseData, error) {
	filePath := path.Join(accountRoot, account, fileName)

	err := os.Remove(filePath)
	if err != nil {
		return common.ResponseData{}, err
	}

	resp := fmt.Sprintf("deleted %s", fileName)
	return createResponseData("DELETE", resp, "", 0, nil, conn), nil
}

// List files under an account
//
// List will fail if the account is not present
func listFiles(account string, conn net.Conn) (common.ResponseData, error) {
	accountPath := path.Join(accountRoot, account)

	files, err := ioutil.ReadDir(accountPath)
	if err != nil {
		return common.ResponseData{}, err
	}

	var size int
	dataList := list.New()
	for _, file := range files {
		byteName := []byte(file.Name())
		dataList.PushBack(common.Data{len(byteName), byteName})
		size += len(byteName)
	}
	common.DebugLog("size: %d\n", uint64(size))
	return createResponseData("LIST", "got list", "", uint64(size), dataList, conn), nil
}

// Send the response and close the connection
func sendResponse(response common.ResponseData) {
	serializedHeader := common.SerializeHeader(response.Header)
	if err := common.SendMessage(serializedHeader, response.DataList, response.Conn); err != nil {
		log.Printf("ERROR: Failed to send message: %v\n", err)
	}
	common.DebugLog("Sent response: %v\n", response.Header)

	// close the connection
	if err := response.Conn.Close(); err != nil {
		log.Printf("ERROR: unable to close connection: %v\n", err)
	}
}

// initialize all workers for server communication
func initServer(address string) (Server, error) {
	var s Server
	var err error

	s.listener, err = net.Listen("tcp", address)
	if err != nil {
		return s, err
	}

	s.handleChan = make(chan net.Conn)
	s.ioChan = make(chan common.ClientData)
	s.respChan = make(chan common.ResponseData)

	log.Printf("Creating conn workers\n")
	for i := 0; i < handleConnWorkers; i++ {
		go func(svr Server) {
			for conn := range svr.handleChan {
				err := handleConnection(conn, s)
				if err != nil {
					log.Printf(err.Error())
					svr.respChan <- createResponseData("ERROR", err.Error(), "", 0, nil, conn)
				}
			}
		}(s)
	}

	log.Printf("Creating io workers\n")
	for i := 0; i < ioWorkers; i++ {
		go func(svr Server) {
			for data := range svr.ioChan {
				err := handleIO(data, s)
				if err != nil {
					log.Printf(err.Error())
					svr.respChan <- createResponseData("ERROR", err.Error(), "", 0, nil, data.Conn)
				}
			}
		}(s)
	}

	log.Printf("Creating response workers\n")
	for i := 0; i < respWorkers; i++ {
		go func(svr Server) {
			for resp := range svr.respChan {
				sendResponse(resp)
			}
		}(s)
	}

	return s, nil
}

func main() {
	port := flag.String("port", defaultPort, "port to listen for connections")
	common.AddCommonFlags()
	flag.Parse()

	log.Printf("port: %s\n", *port)

	// create address
	address := ":" + *port

	server, err := initServer(address)
	if err != nil {
		log.Fatalf("Failed to create server: %v\n", err)
	}

	log.Printf("listening for connections on %s...\n", address)
	// listen for incoming connections
	for {
		connection, err := server.listener.Accept()
		if err != nil {
			log.Fatalf("Failed to accept connection: %v\n", err)
		}

		log.Printf("Received connection from %s\n",
			connection.RemoteAddr().String())
		// spawn a go routine to handle the connection
		server.handleChan <- connection
	}
}
