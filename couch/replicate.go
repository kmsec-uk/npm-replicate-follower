package couch

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"sync/atomic"
	"time"
)

type CouchDocumentChange struct {
	Seq     int             `json:"seq"`
	ID      string          `json:"id"`
	Changes []CouchRevision `json:"changes"`
	Deleted bool            `json:"deleted,omitempty"`
}

// check if a specific revision is present in the list of changes from CouchDB
func (c CouchDocumentChange) HasRevision(rev string) bool {
	for _, change := range c.Changes {
		if change.Rev == rev {
			return true
		}
	}
	return false
}

type CouchRevision struct {
	Rev string `json:"rev"`
}

type CouchResponse struct {
	Results      []CouchDocumentChange `json:"results"`
	LastSequence uint64                `json:"last_seq"`
}

// Result is what the Follower returns while connected
type Result struct {
	Change CouchDocumentChange
	Error  error
}

type Follower struct {
	Client          *http.Client
	Sequence        atomic.Uint64
	userAgent       string
	pollingInterval time.Duration
}

var ErrInvalidUpdateSequence error = errors.New("invalid update sequence")

const (
	defaultUserAgent  string = "npm-replicate-client (go)"
	replicateRegistry string = "https://replicate.npmjs.com/registry/"
)

// creates a new Follower instance
// by default, the follower excludes deletion events
func NewFollower() *Follower {
	return &Follower{
		Client: &http.Client{
			Timeout: 5 * time.Second,
		},
		userAgent:       defaultUserAgent,
		pollingInterval: 2 * time.Second,
	}
}

// use a custom user agent
func (s *Follower) WithUserAgent(ua string) *Follower {
	s.userAgent = ua
	return s
}

// sets the http client timeout to a given time.Duration.
func (s *Follower) WithClientTimeout(t time.Duration) *Follower {
	s.Client.Timeout = t
	return s
}

// set the polling interval for the follower. Default is 2 seconds
// which is more than frequent enough to capture all events
func (s *Follower) WithPollingInterval(t time.Duration) *Follower {
	s.pollingInterval = t
	return s
}

// optionally start from a given sequence as uint64 -- otherwise
// Follower starts from current (most recent) sequence
func (s *Follower) Since(sequence uint64) *Follower {
	s.Sequence.Store(sequence)
	return s
}

// connect and start issuing Results to channel.
func (s *Follower) Connect(ctx context.Context) <-chan Result {

	out := make(chan Result, 10)
	// if we haven't been given a sequence to start with, do cold start
	if s.Sequence.Load() == 0 {
		err := s.coldStartSequence(ctx)
		if err != nil {
			go func() {
				out <- Result{Error: fmt.Errorf("cold start failed: %w", err)}
				close(out)
			}()
			return out
		}
	}

	go func() {
		defer close(out)
		ticker := time.NewTicker(s.pollingInterval)
		defer ticker.Stop()

		fetch := func() {
			// hard-stop 5 second context timeout
			reqCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
			defer cancel()

			changes, err := s.getChanges(reqCtx)
			if err != nil {
				select {
				case out <- Result{Error: err}:
				case <-ctx.Done():

				}
				return
			}

			for _, change := range changes {
				select {
				case out <- Result{Change: change}:
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

// get changes from _changes and return the whole couch result body.
// the sequence is updated in this func
func (s *Follower) getChanges(ctx context.Context) ([]CouchDocumentChange, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", replicateRegistry+"_changes", nil)
	if err != nil {
		return nil, fmt.Errorf("sequence %v: creating request: %w", s.Sequence.Load(), err)
	}
	// user-agent
	req.Header.Add("user-agent", s.userAgent)
	// sequence
	q := req.URL.Query()
	q.Add("since", strconv.FormatUint(s.Sequence.Load(), 10))
	req.URL.RawQuery = q.Encode()

	res, err := s.Client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("sequence %v: doing request: %w", s.Sequence.Load(), err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("sequence %v: unexpected status %v from %s", s.Sequence.Load(), res.StatusCode, res.Request.URL)
	}
	var cr CouchResponse
	err = json.NewDecoder(res.Body).Decode(&cr)
	// fmt.Printf("got %d updates", len(cr.Results))
	if err != nil {
		return nil, fmt.Errorf("sequence %v: decoding body: %w", s.Sequence.Load(), err)
	}
	// update sequence
	_ = s.Sequence.Swap(cr.LastSequence)
	return cr.Results, nil
}

// sets the sequence for CouchDB from a cold start.
// gets the most recent sequence to begin scraping.
func (s *Follower) coldStartSequence(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, "GET", replicateRegistry, nil)
	if err != nil {
		return fmt.Errorf("creating request: %w", err)
	}
	req.Header.Add(
		"user-agent", s.userAgent,
	)
	res, err := s.Client.Do(req)
	if err != nil {
		return fmt.Errorf("doing request: %w", err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status %v from %s", res.StatusCode, res.Request.URL)
	}
	var body struct {
		UpdateSequence uint64 `json:"update_seq"`
	}
	err = json.NewDecoder(res.Body).Decode(&body)

	if err != nil {
		return fmt.Errorf("decoding body: %w", err)
	}
	if body.UpdateSequence == 0 {
		return ErrInvalidUpdateSequence
	}

	s.Sequence.Store(body.UpdateSequence)
	log.Printf("cold start: set sequence to %d\n", body.UpdateSequence)
	return nil
}
