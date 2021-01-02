package client

import (
	"crypto/tls"
	rua "github.com/taoxinyi/rua/framework"
	"io"
	"io/ioutil"
	"net/http"
)

// netHttpClient uses net.Http.Client for the requests
type netHttpClient struct {
	client  *http.Client
	request *http.Request
}

// NewNetHttpClient returns a new netHttpClient
// the actual construction is implemented in Init
func NewNetHttpClient() *netHttpClient {
	return &netHttpClient{}
}

func (c *netHttpClient) Name() string {
	return "net"
}

func (c *netHttpClient) Init(config *rua.LgConfig, request *rua.Request) (err error) {
	client := &http.Client{
		Timeout: config.Timeout,
		Transport: &http.Transport{
			MaxIdleConnsPerHost: config.Connections,
			TLSClientConfig:     &tls.Config{},
		}}
	c.client = client
	c.request = request.HttpRequest
	return nil
}

func (c *netHttpClient) CreateUser() (rua.User, error) {
	return &netHttpUser{client: c.client, request: c.request}, nil
}

// a netHttpUser just grab a connection from the http.Client and send a requests, and wait for a response
type netHttpUser struct {
	client  *http.Client
	request *http.Request
}

func (u *netHttpUser) DoStaticRequest(response *rua.Response) (err error) {
	resp, err := u.client.Do(u.request)
	if err != nil {
		return err
	}
	response.StatusCode = resp.StatusCode
	// not accurate, only calculated body
	// discard the body
	n, err := io.Copy(ioutil.Discard, resp.Body)
	if err != nil {
		return err
	}
	response.Size = int(n)
	return resp.Body.Close()
}
