package client

import (
	"bytes"
	"crypto/tls"
	"errors"
	"fmt"
	rua "github.com/taoxinyi/rua/framework"
	"net"
	"net/url"
	"time"
)

// the bytes for essential http parsing
const bCr byte = '\r'

var bCrlfCrlf = []byte("\r\n\r\n")
var bContentLength = []byte("Content-Length")

// rawHttpClient direct operates on TCP connections and parse TCP data from net.Conn for Http requests
type rawHttpClient struct {
	urlString       string
	maxResponseSize int
	requestBytes    []byte
	timeout         time.Duration
}

// NewRawHttpClient returns a new rawHttpClient
// the actual construction is implemented in Init
func NewRawHttpClient() *rawHttpClient {
	return &rawHttpClient{}
}

func (c *rawHttpClient) Name() string {
	return "raw"
}

func (c *rawHttpClient) Init(config *rua.LgConfig, request *rua.Request) (err error) {
	c.urlString = config.RequestConfig.URL
	c.maxResponseSize = config.RecvBufSize
	c.requestBytes = request.RawBytes
	c.timeout = config.Timeout
	return nil
}

// CreateUser in rawHttpClient creates a TCP connection
func (c *rawHttpClient) CreateUser() (rua.User, error) {
	u, err := url.Parse(c.urlString)
	if err != nil {
		return nil, err
	}
	hostname := u.Hostname()
	port := u.Port()
	if port == "" {
		port = u.Scheme
	}
	address := fmt.Sprintf("%s:%s", hostname, port)
	var conn net.Conn
	if u.Scheme == "http" {
		conn, err = net.DialTimeout("tcp", address, c.timeout)

	} else {
		conn, err = tls.DialWithDialer(&net.Dialer{Timeout: c.timeout}, "tcp", address, &tls.Config{})
	}
	if err != nil {
		return nil, err
	}
	return &rawHttpUser{
		conn:         conn,
		requestBytes: c.requestBytes,
		timeout:      c.timeout,
		rawResponse:  RawResponse{rawBytes: make([]byte, c.maxResponseSize, c.maxResponseSize)},
	}, nil
}

// rawHttpUser contains a dedicated connection, a dedicated bytes for request
type rawHttpUser struct {
	conn net.Conn
	//requestBytes is the unchanged request in bytes
	requestBytes []byte
	timeout      time.Duration
	rawResponse  RawResponse
}

func (u *rawHttpUser) DoStaticRequest(response *rua.Response) (err error) {
	//set deadline
	deadline := time.Now().Add(u.timeout)
	err = u.conn.SetReadDeadline(deadline)
	if err != nil {
		u.conn.Close()
		return err
	}
	//start write and read
	_, err = u.write(u.requestBytes)
	if err != nil {
		u.conn.Close()
		return err
	}
	err = u.fillResponse(response)
	if err != nil {
		u.conn.Close()
		return err
	}
	return nil
}

// write is used to write bytes b to the underlying net.Conn
// It will keep writing until all bytes in len(b) is written or error occurs
func (u *rawHttpUser) write(b []byte) (n int, err error) {
	n, err = u.conn.Write(b)
	if err != nil {
		return n, err
	}
	for n < len(b) {
		written, err := u.write(b[n:])
		return written + n, err
	}
	return n, err
}

// read wraps net.Conn.read, it only read once and returns just as net.Conn.read
func (u *rawHttpUser) read(b []byte) (n int, err error) {
	return u.conn.Read(b)
}

// fillResponse will read until one http response is finished, or an error occurs
// Current implementation will try to read util has first CRLFCRLF, then parse the status line and Headers to get the
// content length, then read until all content is received
// If rawBytes is full but CRLFCRLF is still not encountered (very long headers) it will throw errors so make sure
// to increase maxResponseSize
func (u *rawHttpUser) fillResponse(response *rua.Response) (err error) {
	rawResponse := u.rawResponse
	b := rawResponse.rawBytes
	// read once
	n, err := u.read(b)
	if err != nil {
		return err
	}
	// reset the parser state for a new response
	rawResponse.ResetState()
	for !rawResponse.CanStartParse(n) {
		if n == len(b) {
			// CRLFCRLF is not encountered but the buffer is full
			return errors.New(fmt.Sprintf("Receiver buffer full, didn't encounter CRLFCRLF after %d bytes", len(b)))
		}
		newRead, err := u.read(b[n:])
		if err != nil {
			return err
		}
		n += newRead
	}
	// CRLFCRLF is encountered
	rawResponse.Parse()
	for !rawResponse.IsBodyComplete(n) {
		// keep reading the body to the buffer but it will not be used
		// since we already get statusCode and content length
		newRead, err := u.read(b)
		if err != nil {
			return err
		}
		n += newRead
	}
	// update the response size
	response.Size = n
	response.StatusCode = rawResponse.StatusCode
	return nil
}

// Response contains a status code, the size, content-length as well as the underlying bytes
type RawResponse struct {
	StatusCode    int
	ContentLength int
	// the underlying rawBytes buffer
	rawBytes []byte
	//bodyStart is the index of rawBytes indicating where the body starts
	bodyStart int
	// lastIndex is the last possible index that can start with bCrlfCrlf
	lastIndex int
}

// CanStartParse returns whether the rawBytes given the length, contains CRLFCRLF so it can be parsed
// once CRLFCRLF is found, it will record the location for where the body starts
func (r *RawResponse) CanStartParse(length int) bool {
	i := bytes.Index(r.rawBytes[r.lastIndex:length], bCrlfCrlf)
	if i == -1 {
		// no CRLFCRLF, move cursor ahead of 3 bytes in case CR,LF,CR are already received
		r.lastIndex = intMax(r.lastIndex-3, 0)
		return false
	}
	// CRLFCRLF found, body start is the offset from rawBytes
	r.bodyStart = r.lastIndex + i + 4
	return true
}

// Parse is used to parse the underlying rawBytes to get StatusCode and ContentLength
func (r *RawResponse) Parse() {
	//rua 200 OK status from [9:12),
	r.StatusCode = parseStatusCode(r.rawBytes[9:12])
	headerStart := bytes.IndexByte(r.rawBytes[12:], bCr) + 2
	r.updateContentLengthFromHeaders(r.rawBytes[12+headerStart : r.bodyStart-2])
}

// updateContentLengthFromHeaders is used to parse the headers given the header bytes, and update the ContentLength of the Response
func (r *RawResponse) updateContentLengthFromHeaders(b []byte) {
	for i := bytes.IndexByte(b, bCr); i != -1; i= bytes.IndexByte(b, bCr){
		line := b[:i]
		sep := bytes.IndexByte(b, ':')
		name := line[:sep]
		b = b[i+2:]
		// always assume "Content-Length", case sensitive
		if bytes.Equal(name, bContentLength) {
			// always assume Header is the format "Name: Value"
			value := line[sep+2:]
			// find content length
			r.ContentLength = atoi(value)
			return
		}
	}
	// no content length in the header, default to 0
	r.ContentLength = 0
}

// IsBodyComplete is used to return whether rawBytes is a complete Response given the total number of the bytes read
func (r *RawResponse) IsBodyComplete(length int) bool {
	return length-r.bodyStart == r.ContentLength
}

// ResetState is used to reset the Response state so it can be used for parsing a new one
func (r *RawResponse) ResetState() {
	r.lastIndex = 0
}

// intMax return the max value of two ints
func intMax(x, y int) int {
	if x > y {
		return x
	}
	return y
}

// parseStatusCode assuming status code is always 3 digit
func parseStatusCode(b []byte) int {
	return int(b[0])*100 + int(b[1])*10 + int(b[2]) - 5328
}

// faster atoi assuming content length is at most 4 digits in most cases
func atoi(b []byte) int {
	l := len(b)
	switch l {
	case 1:
		return int(b[0]) - 48
	case 2:
		return int(b[0])*10 + int(b[1]) - 528
	case 3:
		return int(b[0])*100 + int(b[1])*10 + int(b[2]) - 5328
	case 4:
		return int(b[0])*1000 + int(b[1])*100 + int(b[2])*10 + int(b[3]) - 53328
	default:
		res := 0
		for i := 0; i < l; i++ {
			res = 10*res + int(b[i]) - 48
		}
		return res
	}
}