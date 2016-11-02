package nestapi

import (
	"bufio"
	"encoding/json"
	"log"
	"net/http"
	"strings"
)

const (
	// EventTypePut is the event type sent when new data is inserted to the
	// NestAPI instance.
	EventTypePut = "put"
	// EventTypePatch is the event type sent when data at the NestAPI instance is
	// updated.
	EventTypePatch = "patch"
	// EventTypeError is the event type sent when an unknown error is encountered.
	EventTypeError = "event_error"
	// EventTypeAuthRevoked is the event type sent when the supplied auth parameter
	// is no longer valid.
	EventTypeAuthRevoked = "auth_revoked"

	eventTypeKeepAlive  = "keep-alive"
	eventTypeCancel     = "cancel"
	eventTypeRulesDebug = "rules_debug"
)

// Event represents a notification received when watching a
// firebase reference.
type Event struct {
	// Type of event that was received
	Type string
	// Path to the data that changed
	Path string
	// Data that changed
	Data interface{}

	RawData string
}

// Value converts the raw payload of the event into the given interface.
func (e Event) Value(v interface{}) error {
	var tmp struct {
		Data interface{} `json:"data"`
	}
	tmp.Data = &v

	return json.Unmarshal([]byte(e.RawData), &tmp)
}

// StopWatching stops tears down all connections that are watching.
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
// will close the channel given and return nil immediately.
func (n *NestAPI) Watch(notifications chan Event) error {
	if n.isWatching() {
		close(notifications)
		return nil
	}
	// set watching flag
	n.setWatching(true)

	stop := make(chan struct{})
	events, err := n.watch(stop)
	if err != nil {
		return err
	}

	var closedManually bool

	// monitor the stopWatching channel
	// if we're told to stop, close the response Body
	go func() {
		<-n.stopWatching

		closedManually = true
		close(stop)
	}()

	go func() {
		for event := range events {
			if event.Type == EventTypeError && closedManually {
				break
			}

			notifications <- event
		}

		close(notifications)
	}()

	return nil
}

func (n *NestAPI) watch(stop chan struct{}) (chan Event, error) {
	// build SSE request
	req, err := http.NewRequest("GET", n.String(), nil)
	if err != nil {
		n.setWatching(false)
		return nil, err
	}
	req.Header.Add("Accept", "text/event-stream")

	// do request
	resp, err := n.client.Do(req)
	if err != nil {
		n.setWatching(false)
		return nil, err
	}

	notifications := make(chan Event)

	go func() {
		<-stop
		defer resp.Body.Close()
	}()

	// start parsing response body
	go func() {

		// build scanner for response body
		scanner := bufio.NewReader(resp.Body)
		var scanErr error

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
				Type:    strings.Replace(parts[0], "event: ", "", 1),
				RawData: strings.Replace(parts[1], "data: ", "", 1),
			}

			// should be reacting differently based off the type of event
			switch event.Type {
			case EventTypePut, EventTypePatch:
				// we've got extra data we've got to parse
				var data map[string]interface{}
				if err := json.Unmarshal([]byte(strings.Replace(parts[1], "data: ", "", 1)), &data); err != nil {
					scanErr = err
					break scanning
				}

				// set the extra fields
				event.Path = data["path"].(string)
				event.Data = data["data"]

				// ship it
				notifications <- event
			case eventTypeKeepAlive:
				// received ping - nothing to do here
			case eventTypeCancel:
				// The data for this event is null
				// This event will be sent if the Security and NestAPI Rules
				// cause a read at the requested location to no longer be allowed

				// send the cancel event
				notifications <- event
				break scanning
			case EventTypeAuthRevoked:
				// The data for this event is a string indicating that a the credential has expired
				// This event will be sent when the supplied auth parameter is no longer valid
				event.Data = strings.Replace(parts[1], "data: ", "", 1)
				notifications <- event
				break scanning
			case eventTypeRulesDebug:
				log.Printf("Rules-Debug: %s\n", txt)
			}
		}

		if scanErr != nil {
			notifications <- Event{
				Type: EventTypeError,
				Data: scanErr,
			}
		}

		// cleanup routines
		close(notifications)
	}()
	return notifications, nil
}
