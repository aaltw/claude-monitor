package data

import "time"

// Observation is a single data point in the ring buffer.
type Observation struct {
	Timestamp   time.Time
	FiveHourPct float64
	SevenDayPct float64
	TotalTokens int
}

// RingBuffer is a fixed-size circular buffer of observations.
type RingBuffer struct {
	buf  []Observation
	head int
	len  int
	cap  int
}

// NewRingBuffer creates a ring buffer with the given capacity.
func NewRingBuffer(capacity int) *RingBuffer {
	if capacity <= 0 {
		capacity = 1
	}
	return &RingBuffer{
		buf: make([]Observation, capacity),
		cap: capacity,
	}
}

// Add appends an observation, overwriting the oldest if full.
func (rb *RingBuffer) Add(obs Observation) {
	rb.buf[rb.head] = obs
	rb.head = (rb.head + 1) % rb.cap
	if rb.len < rb.cap {
		rb.len++
	}
}

// Len returns the number of observations in the buffer.
func (rb *RingBuffer) Len() int {
	return rb.len
}

// entries returns all observations in chronological order within the given time window.
func (rb *RingBuffer) entries(window time.Duration) []Observation {
	if rb.len == 0 {
		return nil
	}

	cutoff := time.Now().Add(-window)
	var result []Observation

	// Walk the buffer in insertion order (oldest first)
	start := (rb.head - rb.len + rb.cap) % rb.cap
	for i := 0; i < rb.len; i++ {
		idx := (start + i) % rb.cap
		if rb.buf[idx].Timestamp.After(cutoff) {
			result = append(result, rb.buf[idx])
		}
	}
	return result
}

// CalcBurnRate computes burn rate from observations within the given window.
func (rb *RingBuffer) CalcBurnRate(window time.Duration) BurnRate {
	entries := rb.entries(window)
	if len(entries) < 2 {
		return BurnRate{HasData: false}
	}

	first := entries[0]
	last := entries[len(entries)-1]
	elapsed := last.Timestamp.Sub(first.Timestamp).Seconds()

	if elapsed < 1 {
		return BurnRate{HasData: false}
	}

	deltaPct := last.FiveHourPct - first.FiveHourPct
	deltaTokens := float64(last.TotalTokens - first.TotalTokens)

	pctPerHour := (deltaPct / elapsed) * 3600
	tokensPerHour := (deltaTokens / elapsed) * 3600

	var tte time.Duration
	if pctPerHour > 0 {
		hoursLeft := (100.0 - last.FiveHourPct) / pctPerHour
		tte = time.Duration(hoursLeft * float64(time.Hour))
	}

	// Clamp negative rates to zero
	if pctPerHour < 0 {
		pctPerHour = 0
	}
	if tokensPerHour < 0 {
		tokensPerHour = 0
	}

	return BurnRate{
		PercentPerHour: pctPerHour,
		TokensPerHour:  tokensPerHour,
		TimeToExhaust:  tte,
		HasData:        true,
	}
}
