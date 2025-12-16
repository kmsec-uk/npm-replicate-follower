package main

import (
	"context"
	"log"
	"sync"
	"time"

	"github.com/kmsec-uk/npm-follower/couch"
)

func main() {
	ctx := context.Background()
	log.Println("hello from the printer")
	// create the follower
	f := couch.NewFollower().WithPollingInterval(5 * time.Second)
	// connect and start receiving changes from the channel

	var wg sync.WaitGroup
	for event := range f.Connect(ctx) {
		// skip error cases
		if err := event.Error; err != nil {
			log.Printf("error polling: %v\n", err)
			continue
		}

		// skip deletions
		if event.Change.Deleted {
			log.Printf("%s: deleted\n", event.Change.ID)
			continue
		}
		// start goroutine
		wg.Go(func() {
			select {

			case <-time.After(10 * time.Second):
				// wait 10 seconds before getting packument
			case <-ctx.Done():
				return
			}
			// get full packument details
			p, err := f.GetPackument(ctx, &event.Change)
			if err != nil {
				log.Printf("%s: error getting packument: %v\n", event.Change.ID, err)
				return
			}

			// check that the Packument _rev property is aligned with the _changes feed _rev
			if !event.Change.HasRevision(p.Rev) {
				log.Printf("%s: Packument revision (_rev property) %s not in _changes feed. CouchDB says %s", event.Change.ID, p.Rev, event.Change.Changes[0].Rev)
			}
			// do something with the Packument here -- this example gets the latest
			// version (Latest() convenience utility and logs version manifest metadata)
			latest := p.Latest()
			log.Printf("%s: updated - latest version %s was published by npm user %s (%s)", event.Change.ID, latest.Version, latest.NpmUser.Name, latest.NpmUser.Email)
		})

	}
	// Wait for all the 10-second timers to finish before quitting completely
	log.Println("shutting down, waiting for pending workers...")
	wg.Wait()
}
