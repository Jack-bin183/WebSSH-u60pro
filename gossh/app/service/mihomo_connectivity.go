package service

import (
	"context"
	"gossh/gin"
	"net/http"
	"time"
)

// Each browser request owns its probe. There is no background polling or cached
// connectivity result, and all candidate ports share the same three-second budget.
func MihomoConnectivityHandler(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 3*time.Second)
	defer cancel()
	reachable := checkMihomoConnectivity(ctx, mihomoHTTPProxyCandidates(getMihomoDir()), "https://www.google.com/generate_204")
	c.Header("Cache-Control", "no-store")
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": gin.H{"reachable": reachable}})
}

func checkMihomoConnectivity(ctx context.Context, candidates []mihomoHTTPProxyCandidate, target string) bool {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	results := make(chan bool, len(candidates))
	for _, candidate := range candidates {
		go func(proxyURL string) {
			client := mihomoHTTPClientWithProxy(proxyURL)
			defer client.CloseIdleConnections()
			client.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
			req, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
			if err != nil {
				results <- false
				return
			}
			resp, err := client.Do(req)
			if err != nil {
				results <- false
				return
			}
			defer resp.Body.Close()
			results <- resp.StatusCode == http.StatusNoContent
		}(candidate.URL)
	}
	for range candidates {
		select {
		case ok := <-results:
			if ok {
				return true
			}
		case <-ctx.Done():
			return false
		}
	}
	return false
}
