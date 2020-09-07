// Server for journal application
package main

import (
	"bufio"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"os"
	"path"
	"strconv"
	"strings"

	"github.com/teirm/journal_app/protocol"
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

type Server struct {
	listener net.Listener

	handleChan chan net.Conn
	ioChan     chan ClientData
	respChan   chan ResponseData
}

type ClientData struct {
	header protocol.Header
	data   string
	conn   net.Conn
}

type ResponseData struct {
	message string
	conn    net.Conn
}

// Parse the header information read from the client
// connection.
//
// Header Format:
//
//   operation:filename:size
//
//   operation	string
//	 fileName	string
//   size		uint64
//
// Note: Size does not include the size of the header
func parseHeader(header string) (protocol.Header, error) {
	fields := strings.Split(header, ":")
	if len(fields) != headerFields {
		err := fmt.Errorf("invalid header: %s", header)
		return protocol.Header{}, err
	}

	operation := fields[0]
	identity := fields[1]
	fileName := fields[2]
	size, err := strconv.ParseUint(fields[3], 10, 64)
	if err != nil {
		return protocol.Header{}, err
	}

	err = checkOperation(operation)
	if err != nil {
		return protocol.Header{}, err
	}

	return protocol.Header{operation, identity, fileName, size}, nil
}

// Check if the received operation is valid
func checkOperation(operation string) error {
	switch operation {
	case "CREATE":
		return nil
	case "READ":
		return nil
	case "WRITE":
		return nil
	case "DELETE":
		return nil
	case "LIST":
		return nil
	default:
		return fmt.Errorf("Invalid operation: %s", operation)
	}
}

// handle a connection and read client data
// pass client data to io worker
// on error pass error to response worker
func handleConnection(connection net.Conn, svr Server) error {

	input := bufio.NewScanner(connection)
	// parse the header for the opertion type first
	input.Scan()
	if err := input.Err(); err != nil {
		return fmt.Errorf("failed to read header: %v\n", err)
	}

	var clientData ClientData
	header, err := parseHeader(input.Text())
	if err != nil {
		return fmt.Errorf("failed to parse header: %v\n", err)
	}

	clientData.header = header
	clientData.conn = connection

	for input.Scan() {
		if err := input.Err(); err != nil {
			break
		}
		// this is horribly inefficient / dangerous right
		// now but it is easy.
		if input.Text() == "<END>" {
			break
		}
		clientData.data = input.Text()
	}

	if err != nil {
		return fmt.Errorf("error processing input: %v\n", err)
	}

	svr.ioChan <- clientData
	return nil
}

// Perform the requested server side IO operation
// and produce an error or response for the client
func handleIO(data ClientData, svr Server) error {

	op := data.header.Operation

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

	svr.respChan <- ResponseData{res, data.conn}
	return nil
}

// Check if the given path exists or not
func checkExistence(path string) (bool, error) {
	fileInfo, err := os.Stat(path)
	if err == nil {
		return true
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
func createAccount(data ClientData) (string, error) {
	identity := data.header.Identity
	accountPath := path.Join(accountRoot, identity)
	exists, err := checkExistence(accountPath)
	if err != nil {
		return "", err
	}
	if exists == true {
		err := fmt.Errorf("%s already exists", identity)
		return "", err
	}

	err = os.Mkdir(accountPath, defaultPerms)
	if err != nil {
		return "", err
	}

	resp := fmt.Sprintf("account created: %s", identity)
	return resp, nil
}

// Write a file under the given account
//
// Write will fail if the file exists already
func writeFile(data ClientData) (string, error) {
	identity := data.header.Identity
	fileName := data.header.FileName
	filePath := path.Join(accountRoot, identity, fileName)
	exists, err := checkExistence(filePath)
	if err != nil {
		return "", err
	}
	if exists == true {
		err := fmt.Errorf("%s already exists", fileName)
	}

	err = ioutil.WriteFile(filePath, []byte(data.data), defaultPerms)
	if err != nil {
		return "", err
	}

	resp := fmt.Sprintf("wrote file: %s\n", fileName)
	return resp, nil
}

// Send the response and close the connection
func sendResponse(response ResponseData) {
	messageLength := len(response.message)

	for messageLength > 0 {
		bytesSent, err := fmt.Fprintln(response.conn, response.message)
		if err != nil {
			log.Printf("Error: Unable to send message response\n")
			break
		}
		messageLength -= bytesSent
	}
	// close the connection
	if err := response.conn.Close(); err != nil {
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
	s.ioChan = make(chan ClientData)
	s.respChan = make(chan ResponseData)

	log.Printf("Creating conn workers\n")
	for i := 0; i < handleConnWorkers; i++ {
		go func(svr Server) {
			for conn := range svr.handleChan {
				err := handleConnection(conn, s)
				if err != nil {
					log.Printf(err.Error())
					svr.respChan <- ResponseData{err.Error(), conn}
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
					svr.respChan <- ResponseData{err.Error(), data.conn}
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
