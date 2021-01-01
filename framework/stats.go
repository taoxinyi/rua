package framework

import (
	"math"
	"time"
)

type Stats struct {
	// RequestsSent is the total number of requests sent
	// ResponsesRecv is the total number of responses received
	RequestsSent  int64
	ResponsesRecv int64
	// BytesSent is the total number of bytes sent
	// BytesRecv is the total number of bytes received
	BytesSent int64
	BytesRecv int64

	// Latencies is all latency in frequency map, key is microseconds (us, 1/1000ms)
	// Latencies[1234]=3 => 3 requests has latency 1.234 ms

	// MinLatency is the min latency, in microseconds (us, 1/1000ms)
	// MaxLatency is the max latency, in microseconds (us, 1/1000ms)
	Latencies  []int64
	MinLatency int64
	MaxLatency int64

	mean  float64 // LatencyMean of the latency
	stdev float64 // stdev of the latency

	StatusErrors     int64 // error responses status  > 399
	TimeoutErrors    int64 // timeouts
	ConnectionErrors int64 // connections

	limit int64 // upper bound of latency

}

func newStats(timeout time.Duration) *Stats {
	limit := timeout.Microseconds() + 1
	return &Stats{
		limit:      limit,
		Latencies:  make([]int64, limit, limit),
		MinLatency: limit - 1,
	}
}
func (s *Stats) recordRequest(requestSize int64) {
	s.RequestsSent++
	s.BytesSent += requestSize
}
func (s *Stats) recordResponse(latency int64, response *Response) {
	if latency >= s.limit {
		return
	}

	// update received
	s.ResponsesRecv++
	s.BytesRecv += int64(response.Size)

	// verify response code
	if response.StatusCode > 399 {
		s.StatusErrors++
	}

	// update latency
	s.Latencies[latency]++
	if latency < s.MinLatency {
		s.MinLatency = latency
	}
	if latency > s.MaxLatency {
		s.MaxLatency = latency
	}
}

func (s *Stats) mergeStats(other *Stats) {
	s.RequestsSent += other.RequestsSent
	s.ResponsesRecv += other.ResponsesRecv

	s.BytesSent += other.BytesSent
	s.BytesRecv += other.BytesRecv

	s.StatusErrors += other.StatusErrors
	s.TimeoutErrors += other.TimeoutErrors
	s.ConnectionErrors += other.ConnectionErrors

	s.MinLatency = min(s.MinLatency, other.MinLatency)
	s.MaxLatency = max(s.MaxLatency, other.MaxLatency)

	for i := other.MinLatency; i <= other.MaxLatency; i++ {
		s.Latencies[i] += other.Latencies[i]
	}
}
func (s *Stats) LatencyMean() float64 {
	if s.RequestsSent == 0 {
		return 0
	}
	// already calculated
	if s.mean > 0 {
		return float64(s.mean)
	}
	// do calculation
	var sum int64 = 0
	for i := s.MinLatency; i <= s.MaxLatency; i++ {
		sum += i * s.Latencies[i]
	}
	s.mean = float64(sum) / float64(s.ResponsesRecv)
	return s.mean
}
func (s *Stats) LatencyStdev() float64 {
	// not enough data
	if s.ResponsesRecv < 2 {
		return 0
	}
	var sum float64 = 0
	mean := s.LatencyMean()
	for i := s.MinLatency; i <= s.MaxLatency; i++ {
		if s.Latencies[i] > 0 {
			dif := float64(i) - mean
			sum += dif * dif * float64(s.Latencies[i])
		}
	}
	return math.Sqrt(sum / float64(s.ResponsesRecv-1))
}
func (s *Stats) LatencyPercentageWithinStdev(n int) float64 {
	mean := s.LatencyMean()
	stdev := s.LatencyStdev()
	upper := int64(math.Ceil(mean + (float64(n) * stdev)))
	lower := int64(math.Floor(mean - (float64(n) * stdev)))

	var sum int64 = 0
	for i := s.MinLatency; i <= s.MaxLatency; i++ {
		if i >= lower && i <= upper {
			sum += s.Latencies[i]
		}
	}
	return 100.0 * float64(sum) / float64(s.ResponsesRecv)
}
func (s *Stats) LatencyPercentile(percent float64) int64 {
	if percent < 0.0 || percent > 100 {
		return 0
	}
	if percent == 100.0 {
		return s.MaxLatency
	}
	rank := int64(math.Round(percent/100.0*float64(s.RequestsSent) + 0.5))
	var total int64 = 0
	for i := s.MinLatency; i <= s.MaxLatency; i++ {
		total += s.Latencies[i]
		if total >= rank {
			return i
		}
	}
	return 0
}

func max(a, b int64) int64 {
	if a < b {
		return b
	}
	return a
}
func min(a, b int64) int64 {
	if a < b {
		return a
	}
	return b
}
