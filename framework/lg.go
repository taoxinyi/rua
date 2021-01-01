package framework

import (
	"bytes"
	"context"
	"fmt"
	"golang.org/x/sync/errgroup"
	"net/http"
	"net/http/httputil"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"sync/atomic"
	"syscall"
	"time"
)

// HttpClient should store essential parameters, initialize globally shared read only states if needed, and
// create Users with dedicated state for separate goroutines
type HttpClient interface {
	// The name for the HttpClient
	Name() string
	// Init will be called once, with the config it should use and the request it should use
	// It's expected to store essential field in the implementation for the future user creation
	Init(config *LgConfig, request *Request) (err error)
	// CreateUser will be called LgConfig.Connections times
	// Each User object will be used in a separate goroutine, the HttpClient should initialize goroutine specific
	// variables to the User
	CreateUser() (user User, err error)
}

// In each goroutine, a dedicated User will call DoStaticRequest continuously
// once the previous one finished successfully. Since the request is unchanged, HttpClient has the responsibility to
// put the unchanged request as a global read only state so that each User can reference it without creating a
// new one every time
type User interface {
	// DoStaticRequest is used for a request that will not change (immutable)
	// the framework will ask the User to perform the request and the result should be updated
	// in the Response object directly
	// The User should have a reference to the request, instead of creating a new one every time.
	// That's why this interface method don't pass the request object in
	DoStaticRequest(response *Response) (err error)
}

const (
	defaultMethod          = "GET"
	defaultDuration        = time.Second
	defaultConnection      = 1
	defaultTimeout         = time.Minute
	defaultMaxResponseSize = 4096
)

// LgConfig is the configuration for a load generation test
type LgConfig struct {
	// RequestConfig is the configuration of the request a load generation test
	RequestConfig RequestConfig
	// The duration of the entire load generation test
	Duration time.Duration
	// The concurrency level (number of goroutines to be used)
	Connections int
	// The timeout value. Once a connection timeout occurs, that goroutine will be terminated
	Timeout time.Duration
	// the max response size (including status line and headers)
	MaxResponseSize int
	// the verbose level for debugging
	Verbose bool
}

// LgConfig is the configuration for a load generation test
type RequestConfig struct {
	// The HTTP method to be used
	Method string
	// The URL to be used, e.g http://xyz.com/abc?de=fg&hi=jklmn
	URL string
	// The headers to be used
	Headers map[string]string
	// The body to be used. nil if no body is required
	Body []byte
}

// each task is executed in a separate go routine
type task struct {
	// the User of the task
	user User
	// the Dedicated Response for the task
	response *Response
	// the Stats for the task
	stats *Stats
}

type loadGenerator struct {
	// the configuration to be used
	config *LgConfig
	// The underlying HttpClient implementation
	client *HttpClient

	//The Request to be used
	request *Request
	// whether the load generator should stopped
	stop int32
	// all the tasks to be executed, one per goroutine
	tasks []task
}

func setDefaultConfig(config *LgConfig) {
	requestConfig := config.RequestConfig
	if requestConfig.Method == "" {
		requestConfig.Method = defaultMethod
	}
	if config.Duration <= 0 {
		config.Duration = defaultDuration
	}
	if config.Timeout <= 0 {
		config.Timeout = defaultTimeout
	}
	if config.Connections <= 0 {
		config.Connections = defaultConnection
	}
	if config.MaxResponseSize <= 0 {
		config.MaxResponseSize = defaultMaxResponseSize
	}
}

// NewLoadGenerator creates a new Load Generator based on the configuration and the client
// It will generate the Request based on LgConfig.RequestConfig and then
// call HttpClient.Init once
// call HttpClient.CreateUser with LgConfig.Connections times
// finally return the load generator instance
// TODO: add default values for each configuration here
func NewLoadGenerator(config *LgConfig, client HttpClient) (l *loadGenerator, err error) {
	setDefaultConfig(config)
	requestConfig := config.RequestConfig
	request, err := getRequestBytes(requestConfig.Method, requestConfig.URL, requestConfig.Headers, requestConfig.Body)
	if err != nil {
		return nil, err
	}
	if config.Verbose {
		fmt.Printf("Config: %+v\n", *config)
		fmt.Printf("Sending the following HttpRequest with %s\n", client.Name())
		fmt.Println(string(request.RawBytes))
	}

	// handler Init HttpRequest
	err = client.Init(config, request)
	if err != nil {
		return nil, err
	}
	l = &loadGenerator{config: config, request: request}
	// allocate spaces
	l.tasks = make([]task, config.Connections, config.Connections)
	// wait until all finish or first error
	errs, _ := errgroup.WithContext(context.Background())

	for i := 0; i < config.Connections; i++ {
		idx := i
		errs.Go(func() error {
			instance, err := client.CreateUser()
			if err != nil {
				return err
			}
			l.tasks[idx] = task{
				user:     instance,
				response: &Response{},
				stats:    newStats(l.config.Timeout),
			}
			return nil
		})
	}
	return l, errs.Wait()

}

func getRequestBytes(method string, url string, header map[string]string, body []byte) (request *Request, err error) {
	var req *http.Request
	if body != nil {
		req, err = http.NewRequest(method, url, bytes.NewBuffer(body))
	} else {
		req, err = http.NewRequest(method, url, nil)
	}
	if err != nil {
		return nil, err
	}
	if req.ContentLength > 0 {
		req.Header.Add("Content-Length", strconv.FormatInt(req.ContentLength, 10))
	}
	if header != nil {
		for key, value := range header {
			req.Header.Add(key, value)
		}
	}
	rawBytes, err := httputil.DumpRequest(req, body != nil)
	if err != nil {
		return nil, err
	}
	return &Request{HttpRequest: req, RawBytes: rawBytes}, nil
}
func (l *loadGenerator) generateLoadStatic(finishChan chan struct{}, task *task) {
	// initialize a dedicated tv struct for the goroutine
	request := l.request
	requestLen := int64(len(request.RawBytes))
	// new response buffer per goroutine
	response := task.response
	tv := &syscall.Timeval{}
	syscall.Gettimeofday(tv)
	stats := task.stats
	instance := task.user
	for atomic.LoadInt32(&l.stop) == 0 {
		stats.recordRequest(requestLen)
		err := instance.DoStaticRequest(response)
		if err != nil {
			fmt.Println(err)
			// timeout error
			if strings.Contains(strings.ToLower(err.Error()), "timeout") {
				stats.TimeoutErrors++
			} else {
				stats.ConnectionErrors++
			}
			break
		}
		prev := tv.Nano()
		syscall.Gettimeofday(tv)
		latency := (tv.Nano() - prev) / 1e3
		//for _, header := range user.response.Headers() {
		//	fmt.Printf("%s|%s|\n", header.Name, header.Value)
		//}
		//fmt.Println(string(user.response.Body()))

		stats.recordResponse(latency, response)
	}
	finishChan <- struct{}{}
}

// Start the load generator
// It will create LgConfig.Connections goroutines. In each goroutine, a dedicated User created in NewLoadGenerator
// will call User.DoStaticRequest continuously once the previous one finished.
// This function will block until LgConfig.Duration is reached or any error occurs,
// The combined stats for the load generation as well as the actual running time will be returned
func (l *loadGenerator) Start() (finalStats *Stats, actualRunningTime time.Duration) {
	// Stop on Interrupt signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt)

	connections := l.config.Connections
	// make channels for finish
	// TODO maybe use channel of error so the error can be propagated to the caller
	finishChan := make(chan struct{}, connections)
	start := time.Now()

	for i := 0; i < connections; i++ {
		go l.generateLoadStatic(finishChan, &l.tasks[i])
	}

	remaining := connections

	shouldContinue := true

	for shouldContinue {
		select {
		case <-finishChan:
			remaining--
			if remaining == 0 {
				// all channel finished
				shouldContinue = false
			}
		case <-sigChan:
			// received Interrupt signal CTRL+C
			shouldContinue = false
		case <-time.After(l.config.Duration):
			// duration reached
			shouldContinue = false
		}
	}
	// make all channels stop by the signal
	l.Stop()
	// wait all channels stop
	for i := 0; i < remaining; i++ {
		<-finishChan
	}

	// finished
	actualRunningTime = time.Now().Sub(start)

	finalStats = newStats(l.config.Timeout)
	for i := 0; i < connections; i++ {
		finalStats.mergeStats(l.tasks[i].stats)

	}
	return finalStats, actualRunningTime
}

// Stop the load generator
func (l *loadGenerator) Stop() {
	atomic.StoreInt32(&l.stop, 1)
}
