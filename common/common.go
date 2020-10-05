// Common types and methods shared between client and
// server

package common

import (
	"bufio"
	"container/list"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"strconv"
	"strings"
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

const (
	headerFields int = 4
)

var isDebug bool

func AddCommonFlags() {
	flag.BoolVar(&isDebug, "debug", false, "turn on debug messages")
}

func DebugLog(format string, v ...interface{}) {
	if isDebug == true {
		log.Printf(format, v)
	}
}

// Serialize a header into a byte sequence
func SerializeHeader(header Header) []byte {
	sizeStr := strconv.FormatUint(header.Size, 10)
	s := strings.Join([]string{header.Operation, header.Account, header.FileName, sizeStr}, ":")
	return []byte(s + "\n")
}

// Parse the header information beginning every message
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
func ReadHeader(reader *bufio.Reader) (Header, error) {
	header, err := reader.ReadString('\n')
	if err != nil {
		return Header{}, err
	}
	log.Printf("buffer: %s\n", header)
	strippedHeader := strings.TrimSuffix(header, "\n")
	fields := strings.Split(strippedHeader, ":")
	if len(fields) != headerFields {
		err := fmt.Errorf("invalid header: %s", header)
		return Header{}, err
	}

	operation := fields[0]
	identity := fields[1]
	fileName := fields[2]
	size, err := strconv.ParseUint(fields[3], 10, 64)
	if err != nil {
		return Header{}, err
	}

	err = CheckOperation(operation)
	if err != nil {
		return Header{}, err
	}

	return Header{operation, identity, fileName, size}, nil
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

func connWrite(buffer []byte, size int, writer io.Writer) error {
	for size != 0 {
		bytesWritten, err := writer.Write(buffer)
		if err != nil {
			return err
		}
		size -= bytesWritten
	}
	return nil
}

func connRead(reader io.Reader) (Data, error) {
	buffer := make([]byte, 1024)
	bytesRead, err := reader.Read(buffer)
	if err != nil && err != io.EOF {
		return Data{}, err
	}
	return Data{bytesRead, buffer}, err
}

// Common function for reading a message to a connection
func ReadMessage(dataList *list.List, bytesToRead uint64, conn net.Conn) error {
	var err error
	var data Data

	for bytesToRead != 0 {
		data, err = connRead(conn)
		if err != nil && err != io.EOF {
			break
		}
		dataList.PushBack(data)
		bytesToRead -= uint64(data.Size)
	}
	return nil
}

// Common function for writing a message to a connection
func SendMessage(header []byte, dataList *list.List, conn net.Conn) error {
	if err := connWrite(header, len(header), conn); err != nil {
		return err
	}

	if dataList != nil {
		for iter := dataList.Front(); iter != nil; iter = iter.Next() {
			data := iter.Value.(*Data)
			if err := connWrite(data.Buffer, data.Size, conn); err != nil {
				return err
			}
		}
	}
	return nil
}
