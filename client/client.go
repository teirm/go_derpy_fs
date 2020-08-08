package main

import (
	"flag"
	"fmt"
	"log"
	"net"
)

const (
	defaultAddress string = "::"
	defaultPort    string = "0"
)

func main() {
	ip := flag.String("address", defaultAddress, "address to connect to")
	port := flag.String("port", defaultPort, "port to connect to")
	flag.Parse()

	log.Print("ip: %s port: %s\n", *ip, *port)

	address := *ip + ":" + *port

	conn, err := net.Dial("tcp", address)
	if err != nil {
		log.Fatal("Unable to connect to %s: %v\n", address, err)
	}

	fmt.Fprintf(conn, "TEST\n")

	if err := conn.Close(); err != nil {
		log.Fatal("Failed to close connection: %v\n", err)
	}
}
