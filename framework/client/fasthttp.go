package client

import (
	"bufio"
	"bytes"
	rua "github.com/taoxinyi/rua/framework"
	"github.com/valyala/fasthttp"
)

// fastHttpClient uses fasthttp.Client for the requests
type fastHttpClient struct {
	client  *fasthttp.Client
	request *fasthttp.Request
}

// NewFastHttpClient returns a new fastHttpClient
// the actual construction is implemented in Init
func NewFastHttpClient() *fastHttpClient {
	return &fastHttpClient{}
}

func (c *fastHttpClient) Name() string {
	return "fasthttp"
}

// Init creates a client pool and build the request for reuse
func (c *fastHttpClient) Init(config *rua.LgConfig, request *rua.Request) (err error) {
	client := &fasthttp.Client{
		NoDefaultUserAgentHeader:      true,
		MaxConnsPerHost:               config.Connections,
		ReadBufferSize:                config.MaxResponseSize,
		ReadTimeout:                   config.Timeout,
		DisableHeaderNamesNormalizing: true,
	}
	fastRequest := fasthttp.Request{}
	err = fastRequest.Read(bufio.NewReader(bytes.NewBuffer(request.RawBytes)))
	if err != nil {
		return err
	}
	c.client = client
	c.request = &fastRequest
	return nil
}

func (c *fastHttpClient) CreateUser() (rua.User, error) {
	return &fastHttpUser{client: c.client, request: c.request}, nil
}

// a fastHttpUser just grab a connection from the http.Client and send a requests, and wait for a response
type fastHttpUser struct {
	client   *fasthttp.Client
	request  *fasthttp.Request
	response fasthttp.Response
}

func (u *fastHttpUser) DoStaticRequest(response *rua.Response) (err error) {
	err = u.client.Do(u.request, &u.response)
	if err != nil {
		return err
	}
	response.StatusCode = u.response.Header.StatusCode()
	// not accurate, only calculated body
	response.Size = u.response.Header.ContentLength()
	return nil
}
