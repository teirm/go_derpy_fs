// Server for journal application
package main

import (
	"bufio"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net"
)

const (
	defaultPort       string = "0"
	handleConnWorkers int    = 3
	ioWorkers         int    = 3
	respWorkers       int    = 3

	GetOperation Operation = 0
)

type Server struct {
	listener net.Listener

	handleChan chan net.Conn
	ioChan     chan ClientData
	respChan   chan ResponseData
}

type Operation uint64

type ClientData struct {
	op   Operation
	data string
	conn net.Conn
}

type ResponseData struct {
	message string
	conn    net.Conn
}

// parse the header information read from the client
// connection
//
// Header Format:
//   Operation	string
func parseHeader(header string) (Operation, error) {

	switch header {
	case "GET":
		log.Print("Received GET Operation\n")
		return GetOperation, nil
	default:
		return 0, fmt.Errorf("invalid operation: %s", header)
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
	op, err := parseHeader(input.Text())
	if err != nil {
		return fmt.Errorf("failed to parse header: %v\n", err)
	}
	log.Printf("Received OP: %d\n", op)

	clientData.op = op
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

// Do the IO portion
func handleIO(data ClientData, svr Server) error {

	fileName := "/tmp/testfile"
	err := ioutil.WriteFile(fileName, []byte(data.data), 0644)
	if err != nil {
		return err
	}

	message := fmt.Sprintf("Wrote data to %s", fileName)
	response := ResponseData{message, data.conn}
	svr.respChan <- response
	return nil
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

	serverInst, err := initServer(address)
	if err != nil {
		log.Fatalf("Failed to create server: %v\n", err)
	}

	log.Printf("listening for connections on %s...\n", address)
	// listen for incoming connections
	for {
		connection, err := serverInst.listener.Accept()
		if err != nil {
			log.Fatalf("Failed to accept connection: %v\n", err)
		}

		log.Printf("Received connection from %s\n", connection.RemoteAddr().String())
		// spawn a go routine to handle the connection
		serverInst.handleChan <- connection
	}
}
