package data

import (
	"testing"
	"time"
)

func TestRingBuffer_AddAndLen(t *testing.T) {
	rb := NewRingBuffer(5)
	if rb.Len() != 0 {
		t.Fatalf("expected len 0, got %d", rb.Len())
	}

	now := time.Now()
	rb.Add(Observation{Timestamp: now, FiveHourPct: 10.0, SevenDayPct: 20.0, TotalTokens: 1000})
	if rb.Len() != 1 {
		t.Fatalf("expected len 1, got %d", rb.Len())
	}

	// Fill past capacity
	for i := 0; i < 6; i++ {
		rb.Add(Observation{Timestamp: now.Add(time.Duration(i) * time.Second), FiveHourPct: float64(i), TotalTokens: i * 100})
	}
	if rb.Len() != 5 {
		t.Fatalf("expected len 5 (capped at capacity), got %d", rb.Len())
	}
}

func TestRingBuffer_BurnRate_NoData(t *testing.T) {
	rb := NewRingBuffer(100)
	rate := rb.CalcBurnRate(15 * time.Minute)
	if rate.HasData {
		t.Fatal("expected HasData=false with no observations")
	}
}

func TestRingBuffer_BurnRate_SinglePoint(t *testing.T) {
	rb := NewRingBuffer(100)
	rb.Add(Observation{Timestamp: time.Now(), FiveHourPct: 50.0, TotalTokens: 5000})
	rate := rb.CalcBurnRate(15 * time.Minute)
	if rate.HasData {
		t.Fatal("expected HasData=false with only one observation")
	}
}

func TestRingBuffer_BurnRate_LinearGrowth(t *testing.T) {
	rb := NewRingBuffer(100)
	start := time.Now().Add(-10 * time.Minute)

	// 10% over 10 minutes = 60% per hour
	for i := 0; i <= 10; i++ {
		rb.Add(Observation{
			Timestamp:   start.Add(time.Duration(i) * time.Minute),
			FiveHourPct: float64(20 + i), // 20% to 30%
			SevenDayPct: 50.0,
			TotalTokens: 1000 + i*500, // 500 tokens per minute = 30k/hour
		})
	}

	rate := rb.CalcBurnRate(15 * time.Minute)
	if !rate.HasData {
		t.Fatal("expected HasData=true")
	}

	// Should be ~60% per hour (10% in 10 minutes)
	if rate.PercentPerHour < 55 || rate.PercentPerHour > 65 {
		t.Fatalf("expected ~60%%/hour, got %.1f", rate.PercentPerHour)
	}

	// Should be ~30k tokens/hour (5000 tokens in 10 minutes)
	if rate.TokensPerHour < 28000 || rate.TokensPerHour > 32000 {
		t.Fatalf("expected ~30k tokens/hour, got %.0f", rate.TokensPerHour)
	}

	// Time to exhaustion: (100-30)/60 = ~70min
	if rate.TimeToExhaust < 65*time.Minute || rate.TimeToExhaust > 75*time.Minute {
		t.Fatalf("expected ~70min to exhaust, got %v", rate.TimeToExhaust)
	}
}

func TestRingBuffer_BurnRate_ZeroRate(t *testing.T) {
	rb := NewRingBuffer(100)
	start := time.Now().Add(-5 * time.Minute)

	// Flat usage = 0 rate
	for i := 0; i <= 5; i++ {
		rb.Add(Observation{
			Timestamp:   start.Add(time.Duration(i) * time.Minute),
			FiveHourPct: 40.0,
			TotalTokens: 5000,
		})
	}

	rate := rb.CalcBurnRate(15 * time.Minute)
	if !rate.HasData {
		t.Fatal("expected HasData=true")
	}
	if rate.TimeToExhaust != 0 {
		t.Fatalf("expected TimeToExhaust=0 (SAFE) for zero rate, got %v", rate.TimeToExhaust)
	}
}

func TestRingBuffer_BurnRate_WindowFilter(t *testing.T) {
	rb := NewRingBuffer(100)
	now := time.Now()

	// Old data point (30 min ago) - should be excluded from 15min window
	rb.Add(Observation{Timestamp: now.Add(-30 * time.Minute), FiveHourPct: 0.0, TotalTokens: 0})

	// Recent data (last 5 min)
	for i := 0; i <= 5; i++ {
		rb.Add(Observation{
			Timestamp:   now.Add(-5*time.Minute + time.Duration(i)*time.Minute),
			FiveHourPct: float64(50 + i*2), // 50% to 60%
			TotalTokens: 10000 + i*1000,
		})
	}

	rate := rb.CalcBurnRate(15 * time.Minute)
	if !rate.HasData {
		t.Fatal("expected HasData=true")
	}

	// 10% in 5 minutes = 120%/hour (not distorted by old 0% point)
	if rate.PercentPerHour < 110 || rate.PercentPerHour > 130 {
		t.Fatalf("expected ~120%%/hour, got %.1f", rate.PercentPerHour)
	}
}
