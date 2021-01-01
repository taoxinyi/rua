package framework

import (
	"net/http"
)



// Request contains a net.http.Request as well the raw bytes
type Request struct {
	HttpRequest *http.Request
	RawBytes    []byte
}
type Response struct {
	// StatusCode is the Http Status code
	// Size is the total size of the response
	StatusCode    int
	Size          int

}
