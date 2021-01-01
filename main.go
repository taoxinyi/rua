package main

import (
	"errors"
	"fmt"
	flag "github.com/spf13/pflag"
	rua "github.com/taoxinyi/rua/framework"
	"github.com/taoxinyi/rua/framework/client"
	"io/ioutil"
	"os"
	"reflect"
	"runtime"
	"strings"
	"time"
)

const (
	APP     = "rua"
	ERROR   = 1
	SUCCESS = 0
)

type Headers map[string]string

func (h *Headers) Type() string {
	return "string"
}

type Body []byte

func (b Body) Type() string {
	return "string"
}

func (b Body) String() string {
	return string(b)
}

func (b *Body) Set(s string) error {
	body, err := ioutil.ReadFile(s)
	if err != nil {
		return err
	}
	*b = body
	return nil
}

func (h *Headers) String() string {
	return fmt.Sprintf("%s", *h)
}

func (h *Headers) Set(s string) error {
	header := strings.Split(s, ":")
	if len(header) != 2 {
		return errors.New("header must be the format of key: value")
	}
	(*h)[strings.TrimSpace(header[0])] = strings.TrimSpace(header[1])
	return nil
}

var (
	flags *flag.FlagSet

	config  rua.LgConfig
	threads int
	headers Headers = make(map[string]string)
	body    Body

	clients   = make(map[string]rua.HttpClient)
	clientStr string
	version   bool
)

func init() {
	addClient(client.NewRawHttpClient())
	addClient(client.NewFastHttpClient())
	addClient(client.NewNetHttpClient())

	flags = flag.NewFlagSet(APP, flag.ContinueOnError)
	flags.Usage = printUsages
	flags.SortFlags = false

	flags.DurationVarP(&config.Duration, "duration", "d", 10*time.Second, "Duration of test")
	flags.IntVarP(&config.Connections, "connections", "c", 10, "Number of connections")
	flags.IntVarP(&threads, "threads", "t", runtime.NumCPU(), "Number of OS threads to be used")
	flags.VarP(&headers, "header", "H", "HTTP header to add to the request")

	flags.DurationVarP(&config.Timeout, "timeout", "T", 1*time.Second, "Timeout in seconds")
	flags.IntVarP(&config.MaxResponseSize, "max-response-size", "M", 4096, "Max response size in order to allocate buffer")
	flags.StringVarP(&config.RequestConfig.Method, "method", "m", "GET", "The HTTP method to be used")
	flags.VarP(&body, "body", "b", "The file path containing the HTTP body to add to the request")
	flags.StringVarP(&clientStr, "client", "C", "raw", fmt.Sprintf("Use the underlying HTTP client using one of %s", reflect.ValueOf(clients).MapKeys()))

	flags.BoolVarP(&config.Verbose, "verbose", "v", false, "Whether print verbose information")

}
func addClient(client rua.HttpClient) {
	clients[client.Name()] = client
}
func getClient(name string) (client rua.HttpClient, err error) {
	client = clients[name]
	if client == nil {
		return nil, errors.New(fmt.Sprintf("no client with name: %s", name))
	}
	return client, nil
}

func printUsages() {
	fmt.Fprintf(os.Stderr, "Usage: %s <options> url\nOptions:\n", APP)
	flags.PrintDefaults()
}

func main() {
	err := flags.Parse(os.Args)
	// parse failed
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		printUsages()
		os.Exit(ERROR)
	}
	urlStr := flags.Arg(1)
	// no url
	if urlStr == "" {
		fmt.Fprintf(os.Stderr, "url must be provided\n")
		printUsages()
		os.Exit(ERROR)
	}

	// arguments are parsed successfully

	selectedClient, err := getClient(clientStr)
	// no such client
	if err != nil {
		fmt.Println(err)
		printUsages()
		os.Exit(ERROR)
	}

	config.RequestConfig.Headers = headers
	config.RequestConfig.Body = body
	config.RequestConfig.URL = urlStr

	if body != nil && config.RequestConfig.Method == "GET" {
		// GET cannot had body, default to POST
		config.RequestConfig.Method = "POST"
	}
	// create a new lg
	lg, err := rua.NewLoadGenerator(&config, selectedClient)
	if err != nil {
		fmt.Println(err)
		os.Exit(-1)
	}
	fmt.Printf("Running %s test @ %s\n", config.Duration.String(), urlStr)
	fmt.Printf(" %d threads and %d connections\n", threads, config.Connections)
	// set threads, disable profile
	runtime.MemProfileRate = 0
	runtime.GOMAXPROCS(threads)
	stats, actualRunningTime := lg.Start()
	printer.print(stats, actualRunningTime)
}
