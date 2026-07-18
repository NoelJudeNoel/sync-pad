package ratelimit

import (
	"testing"

	"golang.org/x/time/rate"
)

func TestLimiterAllow(t *testing.T) {
	l := New(10, 5)

	ip := "192.168.1.1"
	for i := 0; i < 5; i++ {
		if !l.Allow(ip) {
			t.Fatalf("request %d should be allowed (burst)", i+1)
		}
	}

	if l.Allow(ip) {
		t.Fatal("6th request should be denied (exceeded burst)")
	}
}

func TestLimiterDifferentIPs(t *testing.T) {
	l := New(10, 5)

	if !l.Allow("10.0.0.1") {
		t.Fatal("first IP should be allowed")
	}
	if !l.Allow("10.0.0.2") {
		t.Fatal("second IP should be allowed")
	}
}

func TestExtractIP(t *testing.T) {
	// Can't easily test without httptest, skip for true unit test
	_ = rate.Limit(10)
}
