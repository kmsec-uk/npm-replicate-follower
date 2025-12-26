package registry

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
)

// returns a map[string]string of a user's maintained packages and permissions associated.
// equivalent to GETing https://registry.npmjs.com/-/user/{user}/package
func (c *RegistryClient) GetPackagesForUser(ctx context.Context, user string) (map[string]string, error) {

	// i don't think usernames are permitted to be url unsafe, but let's make it safe anyway
	path := url.PathEscape(user)
	req, err := http.NewRequestWithContext(ctx, "GET", "https://registry.npmjs.com/-/user/"+path+"/package", nil)

	if err != nil {
		return nil, fmt.Errorf("GetPackages: `%s`: creating request: %w", user, err)
	}
	res, err := c.Client.Do(req)

	if err != nil {
		return nil, fmt.Errorf("GetPackages: `%s`: performing request: %w", user, err)
	}
	if res.StatusCode == http.StatusNotFound {
		return nil, ErrPackageNotFound
	}
	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GetPackages: `%s`: unexpected status code %d from %s", user, res.StatusCode, res.Request.URL)
	}
	var m map[string]string
	err = json.NewDecoder(res.Body).Decode(&m)
	if err != nil {
		return nil, fmt.Errorf("GetPackages: `%s`: decoding response: %w", user, err)
	}
	return m, nil
}
