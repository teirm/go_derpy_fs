// Server for journal application
package main

import (
	"bufio"
	"flag"
	"log"
	"net"
)

const (
	defaultPort string = "0"
)

// handle a connection and print out
// values
// TODO(teirm) currently very bare bones
func handleConnection(connection net.Conn) {
	input := bufio.NewScanner(connection)
	for input.Scan() {
		if err := input.Err(); err != nil {
			log.Print("ERROR: processing input: %v\n", err)
			break
		}
		log.Println(input.Text())
	}
	if err := connection.Close(); err != nil {
		log.Print("ERROR: unable to close connection: %v\n", err)
	}
}

func main() {
	port := flag.String("port", defaultPort, "port to listen for connections")
	flag.Parse()

	log.Print("port: %s\n", *port)

	// create address
	address := ":" + *port

	// create a server
	listener, err := net.Listen("tcp", address)
	if err != nil {
		log.Fatal("Failed to create server: %v\n", err)
	}

	log.Print("listening for connections on %s...\n", address)
	// listen for incoming connections
	for {
		connection, err := listener.Accept()
		if err != nil {
			log.Fatal("Failed to accept connection: %v\n", err)
		}

		log.Print("Received connection from %s\n", connection.RemoteAddr().String())
		// spawn a go routine to handle the connection
		go handleConnection(connection)
	}
}
