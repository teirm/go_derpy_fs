// Common types and methods shared between client and
// server

package common

import (
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

// ClientData Information read from the client
type ClientData struct {
	Header Header
	Data   string
	Conn   net.Conn
}

// ResponseData Information to return to the client
type ResponseData struct {
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
