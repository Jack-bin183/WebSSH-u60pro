package service

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestMihomoConnectivity(t *testing.T) {
	for _, status := range []int{http.StatusNoContent, http.StatusOK, http.StatusFound, http.StatusProxyAuthRequired} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			proxy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Host != "probe.invalid" {
					t.Errorf("request did not use the explicit proxy: %s", r.URL)
				}
				w.Header().Set("Location", "http://probe.invalid/redirect")
				w.WriteHeader(status)
			}))
			defer proxy.Close()
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()
			got := checkMihomoConnectivity(ctx, []mihomoHTTPProxyCandidate{{URL: proxy.URL}}, "http://probe.invalid/generate_204")
			if got != (status == http.StatusNoContent) {
				t.Fatalf("status %d: reachable = %v", status, got)
			}
		})
	}
}

func TestMihomoConnectivityDeadline(t *testing.T) {
	proxy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
	}))
	defer proxy.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	start := time.Now()
	candidates := []mihomoHTTPProxyCandidate{{URL: proxy.URL}, {URL: proxy.URL}}
	if checkMihomoConnectivity(ctx, candidates, "http://probe.invalid/generate_204") {
		t.Fatal("timed out probe reported success")
	}
	if time.Since(start) > 500*time.Millisecond {
		t.Fatal("candidate probes exceeded shared deadline")
	}
}

func TestMihomoConnectivityNoCandidates(t *testing.T) {
	if checkMihomoConnectivity(context.Background(), nil, "http://probe.invalid/generate_204") {
		t.Fatal("probe without a proxy reported success")
	}
}
