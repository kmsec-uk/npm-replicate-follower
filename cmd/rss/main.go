package main

import (
	"context"
	"log"
	"time"

	"github.com/kmsec-uk/npm-follower/rss"
)

func main() {
	ctx := context.Background()
	// giving the RSS feed a 2 second buffer (62 second interval) is a
	// good way to ensure we poll the next change correctly.
	// I recommend experimenting pushing the limit parameter higher to
	// find a balance between context timeout, feed generation error,
	// and receiving enough events
	f := rss.NewFollower().WithPollingInterval(62 * time.Second).WithLimit(100)
	// set a reasonable timeout since RSS is slow to generate
	f.WithHTTPTimeout(5 * time.Second)
	for result := range f.Connect(ctx) {
		if err := result.Error; err != nil {
			log.Println(err)
			continue
		}
		log.Println(result.FeedItem.String())
	}
}
