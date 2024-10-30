package httpchecker

import (
	"fmt"
	"net/http"
	"time"
)

// UrlHTTPChecker implements the HTTPChecker interface.
type UrlHTTPChecker struct {
	Client *http.Client
}

// NewHTTPChecker returns an implementation of HTTPChecker with optional HTTP client configuration.
//
//go:generate mockgen -destination=mocks/httpchecker.go -package=mocks github.com/openshift/ocm-agent/pkg/httpchecker HTTPChecker
func NewHTTPChecker(client *http.Client) HTTPChecker {
	// If no client is provided, use a default one with a 10-second timeout
	if client == nil {
		client = &http.Client{
			Timeout: 10 * time.Second,
		}
	}

	// Return an instance of UrlHTTPChecker, which uses the provided client
	return &UrlHTTPChecker{Client: client}
}

// UrlAvailabilityCheck checks if a URL is available by making an HTTP GET request.
func (c *UrlHTTPChecker) UrlAvailabilityCheck(url string) error {
	resp, err := c.Client.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= http.StatusOK && resp.StatusCode < http.StatusMultipleChoices {
		return nil
	}
	return fmt.Errorf("failed to connect to %s with http response code: %d", url, resp.StatusCode)
}
