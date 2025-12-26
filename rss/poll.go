package rss

/*
Disclaimer: the RSS structs are AI-assisted.
*/
import (
	"context"
	"encoding/xml"
	"errors"
	"fmt"
	"net/http"
	"slices"
	"strconv"
	"sync"
	"time"

	"github.com/kmsec-uk/npm-follower/registry"
)

const rssEndpoint string = "https://registry.npmjs.org/-/rss"

var ErrEmptyFeed = errors.New("feed responded with 0 items")

// RSS is the top-level container
type RSSResponse struct {
	Channel Channel `xml:"channel"`
}

// Channel contains metadata and the list of items
type Channel struct {
	Title         string `xml:"title"`
	LastBuildDate string `xml:"lastBuildDate"`
	Items         []Item `xml:"item"`
}

type PubDate string

// Item represents a single entry in the feed
type Item struct {
	Title   string `xml:"title"`
	Link    string `xml:"link"`
	PubDate string `xml:"pubDate"`

	// Namespace handling: Use the full URL, not just the "dc" prefix
	Creator string `xml:"http://purl.org/dc/elements/1.1/ creator"`
}

func (i *Item) String() string {
	return fmt.Sprintf("%s updated by %s - the `latest` dist-tag was released on %s", i.Title, i.Creator, i.PubDate)
}

func (i *Item) Date() (time.Time, error) {
	return time.Parse(time.RFC1123, i.PubDate)
}

// returns true if Item has equal properties to another Item.
func (i *Item) Is(other *Item) bool {
	if i.Creator != other.Creator {
		return false
	}
	if i.Title != other.Title {
		return false
	}
	if i.PubDate != other.PubDate {
		return false
	}
	return true
}

type Result struct {
	FeedItem Item
	Error    error
}

type Follower struct {
	*registry.RegistryClient
	pollingInterval time.Duration
	limit           int
	latest          *Item
	sm              sync.Mutex
}

func NewFollower() *Follower {
	return &Follower{
		RegistryClient:  registry.NewClient(),
		pollingInterval: 2 * time.Second,
		limit:           50,
		latest:          nil,
	}
}

// limit parameter for requesting data from the RSS feed
func (f *Follower) WithLimit(i int) *Follower {
	f.limit = i
	return f
}

func (f *Follower) WithPollingInterval(t time.Duration) *Follower {
	f.pollingInterval = t
	return f
}

// connect and start issuing Results to channel.
func (f *Follower) Connect(ctx context.Context) <-chan Result {

	out := make(chan Result, 10)

	go func() {
		defer close(out)
		ticker := time.NewTicker(f.pollingInterval)
		defer ticker.Stop()

		fetch := func() {
			// hard-stop 10 second context timeout
			reqCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
			defer cancel()

			rssItems, err := f.getChanges(reqCtx)
			if err != nil {
				select {
				case out <- Result{Error: err}:
				case <-ctx.Done():

				}
				return
			}

			for _, item := range rssItems {
				select {
				case out <- Result{FeedItem: item}:
				case <-ctx.Done():
					return
				}
			}

		}

		fetch()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				fetch()
			}
		}
	}()
	return out
}

func (f *Follower) getChanges(ctx context.Context) ([]Item, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", rssEndpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}
	// user-agent
	req.Header.Add("user-agent", f.UserAgent)
	// sequence
	q := req.URL.Query()
	q.Add("descending", "true") // always reverse chronological orders
	q.Add("limit", strconv.Itoa(f.limit))
	req.URL.RawQuery = q.Encode()
	fmt.Println(req.URL.String())
	res, err := f.Client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("doing request: %w", err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status %v from %s", res.StatusCode, res.Request.URL)
	}
	var rr RSSResponse
	err = xml.NewDecoder(res.Body).Decode(&rr)
	if err != nil {
		return nil, fmt.Errorf("decoding body: %w", err)
	}
	if len(rr.Channel.Items) == 0 {
		return nil, ErrEmptyFeed
	}

	truncateIndex := len(rr.Channel.Items)
	// identify truncation point
	if f.latest != nil {
		for idx, item := range rr.Channel.Items {
			if item.Is(f.latest) {
				truncateIndex = idx
				break
			}
		}
	}
	// truncate
	new := rr.Channel.Items[:truncateIndex]

	if len(new) == 0 {
		return []Item{}, nil
	}
	// set latest
	f.sm.Lock()
	latest := rr.Channel.Items[0]
	f.latest = &latest
	f.sm.Unlock()
	// sort
	slices.Reverse(new)
	return new, nil
}
