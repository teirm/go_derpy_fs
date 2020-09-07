// Common types and methods shared between client and
// server

package protocol

type Header struct {
	Operation string
	FileName  string
	Size      uint64
}
