package nestapi

import (
	"bufio"
	"log"
	"strings"
	"sync"
)

// EventTypeError is the type that is set on an Event struct if an
// error occurs while watching a NestAPI reference
const EventTypeError = "event_error"

// Event represents a notification received when watching a
// firebase reference
type Event struct {
	// Type of event that was received
	Type string
	// Data that changed
	Data string
}

// StopWatching stops tears down all connections that are watching
func (n *NestAPI) StopWatching() {
	if n.isWatching() {
		// signal connection to terminal
		n.stopWatching <- struct{}{}
		// flip the bit back to not watching
		n.setWatching(false)
	}
}

func (n *NestAPI) isWatching() bool {
	n.watchMtx.Lock()
	v := n.watching
	n.watchMtx.Unlock()
	return v
}

func (n *NestAPI) setWatching(v bool) {
	n.watchMtx.Lock()
	n.watching = v
	n.watchMtx.Unlock()
}

// Watch listens for changes on a firebase instance and
// passes over to the given chan.
//
// Only one connection can be established at a time. The
// second call to this function without a call to n.StopWatching
// will close the channel given and return nil immediately
func (n *NestAPI) Watch(notifications chan Event) error {
	if n.isWatching() {
		close(notifications)
		return nil
	}
	// set watching flag
	n.setWatching(true)

	// build SSE request
	req, err := n.makeRequest("GET", nil)
	if err != nil {
		n.setWatching(false)
		return err
	}
	req.Header.Add("Accept", "text/event-stream")

	// do request
	resp, err := n.client.Do(req)
	if err != nil {
		n.setWatching(false)
		return err
	}

	// start parsing response body
	go func() {
		// build scanner for response body
		scanner := bufio.NewReader(resp.Body)
		var (
			scanErr        error
			closedManually bool
			mtx            sync.Mutex
		)

		// monitor the stopWatching channel
		// if we're told to stop, close the response Body
		go func() {
			<-n.stopWatching

			mtx.Lock()
			closedManually = true
			mtx.Unlock()

			resp.Body.Close()
		}()
	scanning:
		for scanErr == nil {
			// split event string
			// 		event: put
			// 		data: {"path":"/","data":{"foo":"bar"}}

			var evt []byte
			var dat []byte
			isPrefix := true
			var result []byte

			// For possible results larger than 64 * 1024 bytes (MaxTokenSize)
			// we need bufio#ReadLine()
			// 1. step: scan for the 'event:' part. ReadLine() oes not return the \n
			// so we have to add it to our result buffer.
			evt, isPrefix, scanErr = scanner.ReadLine()
			if scanErr != nil {
				break scanning
			}
			result = append(result, evt...)
			result = append(result, '\n')

			// 2. step: scan for the 'data:' part. NestAPI returns just one 'data:'
			// part, but the value can be very large. If we exceed a certain length
			// isPrefix will be true until all data is read.
			for {
				dat, isPrefix, scanErr = scanner.ReadLine()
				if scanErr != nil {
					break scanning
				}
				result = append(result, dat...)
				if !isPrefix {
					break
				}
			}
			// Again we add the \n
			result = append(result, '\n')
			_, _, scanErr = scanner.ReadLine()
			if scanErr != nil {
				break scanning
			}

			txt := string(result)
			parts := strings.Split(txt, "\n")

			// create a base event
			event := Event{
				Type: strings.Replace(parts[0], "event: ", "", 1),
			}

			// should be reacting differently based off the type of event
			switch event.Type {
			case "put", "patch": // we've got extra data we've got to parse
				event.Data = strings.Replace(parts[1], "data: ", "", 1)
				// ship it
				notifications <- event
			case "keep-alive":
				// received ping - nothing to do here
			case "cancel":
				// The data for this event is null
				// This event will be sent if the Security and NestAPI Rules
				// cause a read at the requested location to no longer be allowed

				// send the cancel event
				notifications <- event
				break scanning
			case "auth_revoked":
				// The data for this event is a string indicating that a the credential has expired
				// This event will be sent when the supplied auth parameter is no longer valid
				notifications <- event
				log.Printf("Auth-Revoked: %s\n", txt)
				break scanning
			case "rules_debug":
				log.Printf("Rules-Debug: %s\n", txt)
			}
		}

		// check error type
		mtx.Lock()
		closed := closedManually
		mtx.Unlock()
		if !closed && scanErr != nil {
			notifications <- Event{
				Type: EventTypeError,
				Data: scanErr.Error(),
			}
		}

		// call stop watching to reset state and cleanup routines
		n.StopWatching()
		close(notifications)

	}()
	return nil
}
