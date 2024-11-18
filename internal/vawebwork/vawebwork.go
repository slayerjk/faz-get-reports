package vawebwork

import (
	"crypto/tls"
	"net/http"
	"time"
)

// Create http insecure client
//
// InsecureSkipVerify: true
func NewInsecureClient() http.Client {
	tlsConfig := &tls.Config{
		InsecureSkipVerify: true,
	}

	transport := &http.Transport{
		TLSClientConfig: tlsConfig,
		MaxIdleConns:    10,
		IdleConnTimeout: 30 & time.Second,
	}

	client := &http.Client{Transport: transport}

	return *client
}
