package main

import (
	"fmt"
	"github.com/dustin/go-humanize"
	"github.com/olekukonko/tablewriter"
	rua "github.com/taoxinyi/rua/framework"
	"os"
	"strings"
	"time"
)

type Printer struct{}

var printer Printer

func (p *Printer) print(stats *rua.Stats, duration time.Duration) {
	seconds := duration.Seconds()
	var headers []string
	var data [][]string

	headers = []string{"", "Connection", "Timeout", "Status"}
	data = [][]string{{
		"Errors",
		fmt.Sprintf("%d", stats.ConnectionErrors),
		fmt.Sprintf("%d", stats.TimeoutErrors),
		fmt.Sprintf("%d", stats.StatusErrors),
	}}
	printTable(headers, data)

	headers = []string{"", "Avg", "Min", "Max", "tdev", "+/- Stdev"}
	data = [][]string{{
		"Latency",
		fmt.Sprintf("%.3fms", float64(stats.LatencyMean())/1000.0),
		fmt.Sprintf("%.3fms", float64(stats.MinLatency)/1000.0),
		fmt.Sprintf("%.3fms", float64(stats.MaxLatency)/1000.0),
		fmt.Sprintf("%.3fms", stats.LatencyStdev()/1000.0),
		fmt.Sprintf("%.3f%%", stats.LatencyPercentageWithinStdev(1)),
	}}
	printTable(headers, data)

	headers = []string{"", "50%", "75%", "90%", "99%", "99.9%"}
	data = [][]string{{
		"Latency",
		fmt.Sprintf("%.3fms", float64(stats.LatencyPercentile(50))/1000.0),
		fmt.Sprintf("%.3fms", float64(stats.LatencyPercentile(75))/1000.0),
		fmt.Sprintf("%.3fms", float64(stats.LatencyPercentile(90))/1000.0),
		fmt.Sprintf("%.3fms", float64(stats.LatencyPercentile(99))/1000.0),
		fmt.Sprintf("%.3fms", float64(stats.LatencyPercentile(99.9))/1000.0),
	}}
	printTable(headers, data)

	headers = []string{"", "Count", "Count/s", "Size", "Throughput"}
	data = [][]string{{
		"Requests",
		fmt.Sprintf("%d", stats.RequestsSent),
		fmt.Sprintf("%.2f", float64(stats.RequestsSent)/seconds),
		fmt.Sprintf("%s", humanize.IBytes(uint64(stats.BytesSent))),
		fmt.Sprintf("%s/s", humanize.IBytes(uint64(float64(stats.BytesSent)/seconds))),
	}, {
		"Responses",
		fmt.Sprintf("%d", stats.ResponsesRecv),
		fmt.Sprintf("%.2f", float64(stats.ResponsesRecv)/seconds),
		fmt.Sprintf("%s", humanize.IBytes(uint64(stats.BytesRecv))),
		fmt.Sprintf("%s/s", humanize.IBytes(uint64(float64(stats.BytesRecv)/seconds))),
	},
	}

	printTable(headers, data)

	fmt.Printf("\n%d responses received in %s, %s read\n", stats.ResponsesRecv, duration, humanize.IBytes(uint64(stats.BytesRecv)))

}
func printTable(headers []string, data [][]string) {
	fmt.Println(strings.Repeat("-", 72))
	table := tablewriter.NewWriter(os.Stdout)
	for i := 0; i < len(headers); i++ {
		table.SetColMinWidth(i, 12)
	}
	table.SetHeader(headers)
	table.SetAutoWrapText(false)
	table.SetAutoFormatHeaders(false)
	table.SetHeaderAlignment(tablewriter.ALIGN_LEFT)
	table.SetAlignment(tablewriter.ALIGN_LEFT)
	table.SetCenterSeparator("")
	table.SetColumnSeparator("")
	table.SetRowSeparator("")
	table.SetHeaderLine(false)
	table.SetBorder(false)
	table.SetNoWhiteSpace(true)
	table.AppendBulk(data) // Add Bulk Data

	table.Render()
}
