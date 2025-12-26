package registry

import (
	"net/http"
	"time"
)

const defaultUserAgent string = "npm-replicate-client (go)"

// RegistryClient handles the interaction with the npm registry api.
type RegistryClient struct {
	Client    *http.Client
	UserAgent string
}

func NewClient() *RegistryClient {
	return &RegistryClient{
		Client:    &http.Client{Timeout: 5 * time.Second},
		UserAgent: defaultUserAgent,
	}
}

func (c *RegistryClient) WithHTTPTimeout(t time.Duration) *RegistryClient {
	c.Client.Timeout = t
	return c
}

func (c *RegistryClient) WithUserAgent(ua string) *RegistryClient {
	c.UserAgent = ua
	return c
}
