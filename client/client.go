// bare bones client implementation
package main

import (
	"bufio"
	"flag"
	"fmt"
	"log"
	"net"
	"strconv"
	"strings"
)

const (
	defaultAddress string = "127.0.0.1"
	defaultPort    string = "0"
)

// Create a header
func createHeader(op string, account string, fileName string, size uint64) string {
	sizeStr := strconv.FormatUint(size, 64)
	s := []string{op, account, fileName, sizeStr}

	return strings.Join(s, ":")
}

func main() {
	ip := flag.String("address", defaultAddress, "address to connect to")
	port := flag.String("port", defaultPort, "port to connect to")
	flag.Parse()

	log.Printf("ip: %s port: %s\n", *ip, *port)

	address := *ip + ":" + *port

	conn, err := net.Dial("tcp", address)
	if err != nil {
		log.Fatalf("Unable to connect to %s: %v\n", address, err)
	}

	fmt.Fprintf(conn, "GET:/tmp/test:6\nFoobar\n<END>\n")

	// read any response from the server
	input := bufio.NewScanner(conn)
	for input.Scan() {
		if err := input.Err(); err != nil {
			break
		}
		fmt.Println(input.Text())
	}
	if err != nil {
		log.Fatalf("Error reading from connection: %v\n", err)
	}

	if err := conn.Close(); err != nil {
		log.Fatalf("Failed to close connection: %v\n", err)
	}
}
