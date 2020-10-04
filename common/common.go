// Common types and methods shared between client and
// server

package common

import (
	"container/list"
	"fmt"
	"net"
)

// Header information describing client data
type Header struct {
	Operation string
	Account   string
	FileName  string
	Size      uint64
}

type Data struct {
	Size   int
	Buffer []byte
}

// ClientData Information read from the client
type ClientData struct {
	Header   Header
	DataList *list.List
	Conn     net.Conn
}

// ResponseData Information to return to the client
type ResponseData struct {
	// TODO: can this also be a list? will it write to the socket?
	// Gut says no since it would just be a pointer.
	// What can be done is it can be a list and then the
	// Write method writes out each buffer -- client needs
	// to then assemble the message.
	// Simplest thing to do would be to have 1 / n connections
	// for client -- if they exceed that can be a rejection
	// or a buffer
	Message string
	Conn    net.Conn
}

// Check if the received operation is valid
func CheckOperation(operation string) error {
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
