// Server for journal application
package main

import (
	"bufio"
	"container/list"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"os"
	"path"
	"strconv"
	"strings"

	"github.com/teirm/go_ftp/common"
)

const (
	defaultPort string = "0"

	handleConnWorkers int = 3
	ioWorkers         int = 3
	respWorkers       int = 3

	headerFields int    = 3
	headerDelim  string = ":"

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

// Parse the header information read from the client
// connection.
//
// Header Format:
//
//   operation:account:filename:size
//
//   operation	string
//   account	string
//	 fileName	string
//   size		uint64
//
// Note: Size does not include the size of the header
func parseHeader(connReader *bufio.Reader) (common.Header, error) {

	header, err := connReader.ReadString('\n')
	if err != nil {
		return common.Header{}, err
	}
	fields := strings.Split(header, ":")
	if len(fields) != headerFields {
		err := fmt.Errorf("invalid header: %s", header)
		return common.Header{}, err
	}

	operation := fields[0]
	identity := fields[1]
	fileName := fields[2]
	size, err := strconv.ParseUint(fields[3], 10, 64)
	if err != nil {
		return common.Header{}, err
	}

	err = common.CheckOperation(operation)
	if err != nil {
		return common.Header{}, err
	}

	return common.Header{operation, identity, fileName, size}, nil
}

// handle a connection and read client data
// pass client data to io worker
// on error pass error to response worker
func handleConnection(connection net.Conn, svr Server) error {
	connReader := bufio.NewReader(connection)
	header, err := parseHeader(connReader)
	if err != nil {
		return fmt.Errorf("failed to parse header: %v", err)
	}

	var message common.ClientData
	message.Header = header
	message.Conn = connection
	message.DataList = list.New()

	var bytesToRead = message.Header.Size
	for bytesToRead != 0 {
		buffer := make([]byte, 1024)
		bytesRead, err := connReader.Read(buffer)
		if err != nil {
			break
		}
		message.DataList.PushBack(common.Data{bytesRead, buffer})
		bytesToRead -= uint64(bytesRead)

	}

	if err != nil {
		return fmt.Errorf("error processing input: %v", err)
	}

	svr.ioChan <- message
	return nil
}

// Perform the requested server side IO operation
// and produce an error or response for the client
func handleIO(data common.ClientData, svr Server) error {

	op := data.Header.Operation

	var res string
	var err error
	switch op {
	case "CREATE":
		res, err = createAccount(data)
	case "WRITE":
		res, err = writeFile(data)
	case "READ":
		res, err = readFile(data)
	case "DELETE":
		res, err = deleteFile(data)
	case "LIST":
		res, err = listFiles(data)
	default:
		// probably should be a panic -- this would be a programmer
		// error
		return fmt.Errorf("Invalid operation: %s", op)
	}
	if err != nil {
		return err
	}

	svr.respChan <- common.ResponseData{res, data.Conn}
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
func createAccount(data common.ClientData) (string, error) {
	account := data.Header.Account
	accountPath := path.Join(accountRoot, account)
	exists, err := checkExistence(accountPath)
	if err != nil {
		return "", err
	}
	if exists == true {
		err := fmt.Errorf("%s already exists", account)
		return "", err
	}

	err = os.Mkdir(accountPath, defaultPerms)
	if err != nil {
		return "", err
	}

	resp := fmt.Sprintf("account created: %s", account)
	return resp, nil
}

// Write a file under the given account
//
// Write will fail if the file exists already
func writeFile(data common.ClientData) (string, error) {
	account := data.Header.Account
	fileName := data.Header.FileName
	filePath := path.Join(accountRoot, account, fileName)

	file, err := os.OpenFile(filePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, defaultPerms)
	if err != nil {
		return "", err
	}

	for iter := data.DataList.Front(); iter != nil; iter = iter.Next() {
		// TODO: ick on so many levels -- maybe write own linked list
		fileData := iter.Value.(*common.Data)
		if _, err := file.Write(fileData.Buffer); err != nil {
			file.Close()
			return "", err
		}
	}

	resp := fmt.Sprintf("wrote file: %s\n", fileName)
	return resp, nil
}

// Read a file under the given account
//
// Read will fail if the file does not exist
func readFile(data common.ClientData) (string, error) {
	account := data.Header.Account
	fileName := data.Header.FileName
	filePath := path.Join(accountRoot, account, fileName)

	fileData, err := ioutil.ReadFile(filePath)
	if err != nil {
		return "", err
	}

	//TODO if the data is not a string -- this is wrong
	return string(fileData), nil
}

// Delete a file under the given account
//
// Delete will fial if the file does not exist
func deleteFile(data common.ClientData) (string, error) {
	account := data.Header.Account
	fileName := data.Header.FileName
	filePath := path.Join(accountRoot, account, fileName)

	err := os.Remove(filePath)
	if err != nil {
		return "", err
	}

	resp := fmt.Sprintf("deleted %s", fileName)
	return resp, nil
}

// List files under an account
//
// List will fail if the account is not present
func listFiles(data common.ClientData) (string, error) {
	account := data.Header.Account
	accountPath := path.Join(accountRoot, account)

	files, err := ioutil.ReadDir(accountPath)
	if err != nil {
		return "", err
	}

	var resp string
	for _, file := range files {
		resp = resp + file.Name() + "\n"
	}
	return resp, nil
}

// Send the response and close the connection
func sendResponse(response common.ResponseData) {
	messageLength := len(response.Message)

	for messageLength > 0 {
		bytesSent, err := fmt.Fprintln(response.Conn, response.Message)
		if err != nil {
			log.Printf("Error: Unable to send message response\n")
			break
		}
		messageLength -= bytesSent
	}
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
					svr.respChan <- common.ResponseData{err.Error(), conn}
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
					svr.respChan <- common.ResponseData{err.Error(), data.Conn}
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
