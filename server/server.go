// Server for journal application
package main

import (
	"bufio"
	"flag"
	"fmt"
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
func handleConnection(connection net.Conn, svr Server) {

	input := bufio.NewScanner(connection)
	// parse the header for the opertion type first
	input.Scan()
	if err := input.Err(); err != nil {
		log.Printf("ERROR: failed to read header: %v\n", err)
		//TODO handle error
	}

	var clientData ClientData
	op, err := parseHeader(input.Text())
	if err != nil {
		log.Printf("ERROR: unable to parse header: %v\n", err)
		//TODO handle error
	}
	log.Printf("Received OP: %d\n", op)

	clientData.op = op

	for input.Scan() {
		if err := input.Err(); err != nil {
			log.Printf("ERROR: processing input: %v\n", err)
			//TODO handle error
			break
		}
		// this is probably horribly inefficient right
		// now but it is easy.
		clientData.data = input.Text()
	}
	svr.ioChan <- clientData
}

// Do the IO portion
func handleIO(data ClientData) {
	log.Print("Unimplemented right now\n")
	log.Print(data.data)
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
	if err := connection.Close(); err != nil {
		log.Printf("ERROR: unable to close connection: %v\n", err)
		//TODO don't close connection and handle error properly
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

	for i := 0; i < handleConnWorkers; i++ {
		go func(svr Server) {
			for conn := range svr.handleChan {
				handleConnection(conn, s)
			}
		}(s)
	}

	for i := 0; i < ioWorkers; i++ {
		go func(svr Server) {
			for data := range svr.ioChan {
				handleIO(data)
			}
		}(s)
	}

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
