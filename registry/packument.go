package registry

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
)

type Contact struct {
	Name  string `json:"name"`
	Email string `json:"email,omitempty"`
	URL   string `json:"url,omitempty"`
}

func (c *Contact) UnmarshalJSON(data []byte) error {
	var obj struct {
		Name  string `json:"name"`
		Email string `json:"email,omitempty"`
		URL   string `json:"url,omitempty"`
	}
	if err := json.Unmarshal(data, &obj); err == nil {
		*c = Contact{
			Name:  obj.Name,
			Email: obj.Email,
			URL:   obj.URL,
		}
		return nil
	}
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}
	*c = Contact{Name: s}
	return nil
}

type Repository struct {
	URL       string `json:"url"`
	Type      string `json:"type,omitempty"`
	Directory string `json:"directory,omitempty"`
}

// packument.repository can be of type string or type Repository. Support both
func (r *Repository) UnmarshalJSON(data []byte) error {
	// Try to unmarshal as object
	var obj struct {
		Type string `json:"type"`
		URL  string `json:"url"`
	}
	if err := json.Unmarshal(data, &obj); err == nil {
		*r = Repository{Type: obj.Type, URL: obj.URL}
		return nil
	}

	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}
	*r = Repository{URL: s}
	return nil
}

type Unpublished struct {
	Time     string   `json:"time"`
	Versions []string `json:"versions"`
}

// packument.time can be of type map[stirng]string
type Time struct {
	Created      string      `json:"created"`
	Modified     string      `json:"modified"`
	Unpublished  Unpublished `json:"unpublished"`
	VersionTimes map[string]string
}

func (t *Time) UnmarshalJSON(data []byte) error {
	raw := make(map[string]json.RawMessage)
	err := json.Unmarshal(data, &raw)
	if err != nil {
		return fmt.Errorf("unmarshalling packument.time: %w", err)
	}
	t.VersionTimes = make(map[string]string)
	for k, v := range raw {
		switch k {
		case "created":
			if err := json.Unmarshal(v, &t.Created); err != nil {
				return fmt.Errorf("unmarshalling created: %w", err)
			}

		case "modified":
			if err := json.Unmarshal(v, &t.Modified); err != nil {
				return fmt.Errorf("unmarshalling modified: %w", err)
			}
		case "unpublished":
			err := json.Unmarshal(v, &t.Unpublished)
			if err != nil {
				return fmt.Errorf("unmarshalling unpublished: %w", err)
			}

		default:
			var val string
			if err := json.Unmarshal(v, &val); err != nil {
				return fmt.Errorf("unmarshalling key %s of time: %w", k, err)
			}
			t.VersionTimes[k] = val
		}
	}
	return nil
}

type Dist struct {
	Tarball string `json:"tarball"`
}

// Packument Version

type Bugs struct {
	Url string `json:"url"`
}

type PackageVersion struct {
	Name       string            `json:"name"`
	Version    string            `json:"version"`
	Dist       Dist              `json:"dist"`
	Author     Contact           `json:"author"`
	Scripts    map[string]string `json:"scripts"`
	Repository Repository        `json:"repository"`
	Homepage   string            `json:"homepage"`
	Bugs       Bugs              `json:"bugs"`
	NpmUser    Contact           `json:"_npmUser"`
}

// Packument
type Packument struct {
	Time        Time                      `json:"time"`
	Versions    map[string]PackageVersion `json:"versions"`
	Homepage    string                    `json:"homepage"`
	Author      *Contact                  `json:"author,omitempty"`
	Description string                    `json:"description,omitempty"`
	Keywords    []string                  `json:"keywords,omitempty"`
	Maintainers []Contact                 `json:"maintainers,omitempty"`
	Name        string                    `json:"name"`
	Readme      string                    `json:"readme,omitempty"`
	Repository  *Repository               `json:"repository,omitempty"`
	DistTags    map[string]string         `json:"dist-tags,omitempty"`
	Rev         string                    `json:"_rev"` // couchdb _rev property
}

var ErrPackageNotFound = errors.New("packument not found")

// returns true if the packument suggests npm have issued
// a holding package (i.e. package taken down)
func (packument *Packument) IsHoldingPackage() bool {
	if packument.Description != "security holding package" {
		return false
	}
	if len(packument.Versions) != 1 {
		return false
	}
	if _, ok := packument.Versions["0.0.1-security"]; !ok {
		return false
	}
	return true
}

// returns the Package Version manifest for the version that
// matches the `latest` dist-tag.
func (packument *Packument) Latest() *PackageVersion {
	for dist, version := range packument.DistTags {
		if dist == "latest" {
			for v, pv := range packument.Versions {
				if version == v {
					return &pv
				}
			}
		}
	}
	// nil condition should never be met, according to shoddy registry docs - dist-tag
	// should *always* have a `latest` value.
	return nil
}

// retrieves the full packument and returns an unmarshalled
// Packument struct.
// Equivalent to GETing https://registry.npmjs.com/{package}
func (c *RegistryClient) GetPackument(ctx context.Context, id string) (*Packument, error) {
	body, err := c.FetchPackument(ctx, id)

	if err != nil {
		return nil, fmt.Errorf("fetching packument for %s: %w", id, err)
	}
	defer body.Close()
	var packument Packument
	err = json.NewDecoder(body).Decode(&packument)
	if err != nil {
		return nil, fmt.Errorf("unmarshalling packument for %s: %w", id, err)
	}

	return &packument, nil
}

// fetches the Packument for a given package name.
// retuns an io.ReadCloser for decoding or reading.
func (c *RegistryClient) FetchPackument(ctx context.Context, id string) (io.ReadCloser, error) {
	packageName := url.PathEscape(id)
	req, err := http.NewRequestWithContext(ctx, "GET", "https://registry.npmjs.com/"+packageName, nil)
	if err != nil {
		return nil, fmt.Errorf("packument fetch: `%s`: creating request: %w", packageName, err)
	}
	res, err := c.Client.Do(req)

	if err != nil {
		return nil, fmt.Errorf("packument fetch: `%s`: performing request: %w", packageName, err)
	}
	if res.StatusCode == http.StatusNotFound {
		return nil, ErrPackageNotFound
	}
	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("packument fetch: `%s`: unexpected status code %d from %s", packageName, res.StatusCode, res.Request.URL)
	}
	return res.Body, nil
}

// returns an unmarshalled Package Version manifest. This is
// equivalent to getting https://registry.npmjs.com/{package}/latest
func (c *RegistryClient) GetLatestVersionManifest(ctx context.Context, id string) (*PackageVersion, error) {
	body, err := c.FetchLatestVersionManifest(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("fetching latest for %s: %w", id, err)
	}
	defer body.Close()
	var manifest PackageVersion
	err = json.NewDecoder(body).Decode(&manifest)
	if err != nil {
		return nil, fmt.Errorf("unmarshalling manifest for %s: %w", id, err)
	}
	return &manifest, nil
}

// fetches the latest version manifest for a given package name.
// retuns an io.ReadCloser for decoding or reading.
func (c *RegistryClient) FetchLatestVersionManifest(ctx context.Context, id string) (io.ReadCloser, error) {
	packageName := url.PathEscape(id)
	req, err := http.NewRequestWithContext(ctx, "GET", "https://registry.npmjs.com/"+packageName+"/latest", nil)
	if err != nil {
		return nil, fmt.Errorf("latest fetch: `%s`: creating request: %w", packageName, err)
	}
	res, err := c.Client.Do(req)

	if err != nil {
		return nil, fmt.Errorf("latest fetch: `%s`: performing request: %w", packageName, err)
	}
	if res.StatusCode == http.StatusNotFound {
		return nil, ErrPackageNotFound
	}
	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("latest fetch: `%s`: unexpected status code %d from %s", packageName, res.StatusCode, res.Request.URL)
	}
	return res.Body, nil
}
