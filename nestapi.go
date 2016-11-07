/*
Package nestapi is a REST client for NestAPI (https://firebase.com).
*/
package nestapi

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	_url "net/url"
	"strings"
	"sync"
	"time"
)

// TimeoutDuration is the length of time any request will have to establish
// a connection and receive headers from NestAPI before returning
// an APIError timeout
var (
	TimeoutDuration               = 120 * time.Second
	KeepAliveTimeoutDuration      = 35 * time.Second
	ResponseHeaderTimeoutDuration = 10 * time.Second
	defaultRedirectLimit          = 30
)

// ErrTimeout is an error type is that is returned if a request
// exceeds the TimeoutDuration configured.
type ErrTimeout struct {
	error
}

// query parameter constants
const (
	authParam = "auth"
)

// NestAPI represents a location in the cloud.
type NestAPI struct {
	url    string
	params _url.Values
	client *http.Client

	eventMtx   sync.Mutex
	eventFuncs map[string]chan struct{}

	watchMtx     sync.Mutex
	watching     bool
	stopWatching chan struct{}
}

func sanitizeURL(url string) string {
	if !strings.HasPrefix(url, "https://") && !strings.HasPrefix(url, "http://") {
		url = "https://" + url
	}

	if strings.HasSuffix(url, "/") {
		url = url[:len(url)-1]
	}

	return url
}

// Preserve headers on redirect.
//
// Reference https://github.com/golang/go/issues/4800
func redirectPreserveHeaders(req *http.Request, via []*http.Request) error {
	if len(via) == 0 {
		// No redirects
		return nil
	}

	if len(via) > defaultRedirectLimit {
		return fmt.Errorf("%d consecutive requests(redirects)", len(via))
	}

	// mutate the subsequent redirect requests with the first Header
	for key, val := range via[0].Header {
		req.Header[key] = val
	}
	return nil
}

// New creates a new NestAPI reference,
// if client is nil, http.DefaultClient is used.
func New(url string, client *http.Client) *NestAPI {

	if client == nil {
		var tr *http.Transport
		tr = &http.Transport{
			ResponseHeaderTimeout: ResponseHeaderTimeoutDuration,
			DialContext: (&net.Dialer{
				Timeout:   TimeoutDuration,
				KeepAlive: KeepAliveTimeoutDuration,
			}).DialContext,
		}

		client = &http.Client{
			Transport:     tr,
			CheckRedirect: redirectPreserveHeaders,
		}
	}

	return &NestAPI{
		url:          sanitizeURL(url),
		params:       _url.Values{},
		client:       client,
		stopWatching: make(chan struct{}),
		eventFuncs:   map[string]chan struct{}{},
	}
}

// Auth sets the custom NestAPI token used to authenticate to NestAPI.
func (n *NestAPI) Auth(token string) {
	n.params.Set(authParam, token)
}

// Unauth removes the current token being used to authenticate to NestAPI.
func (n *NestAPI) Unauth() {
	n.params.Del(authParam)
}

// Set the value of the NestAPI reference.
func (n *NestAPI) Set(v interface{}) error {
	bytes, err := json.Marshal(v)
	if err != nil {
		return err
	}
	_, err = n.doRequest("PUT", bytes)
	return err
}

// String returns the string representation of the
// NestAPI reference.
func (n *NestAPI) String() string {
	path := n.url + "/.json"

	if len(n.params) > 0 {
		path += "?" + n.params.Encode()
	}
	return path
}

// Child creates a new NestAPI reference for the requested
// child with the same configuration as the parent.
func (n *NestAPI) Child(child string) *NestAPI {
	c := n.copy()
	c.url = c.url + "/" + child
	return c
}

func (n *NestAPI) copy() *NestAPI {
	c := &NestAPI{
		url:          n.url,
		params:       _url.Values{},
		client:       n.client,
		stopWatching: make(chan struct{}),
		eventFuncs:   map[string]chan struct{}{},
	}

	// making sure to manually copy the map items into a new
	// map to avoid modifying the map reference.
	for k, v := range n.params {
		c.params[k] = v
	}
	return c
}

func (n *NestAPI) doRequest(method string, body []byte) ([]byte, error) {
	req, err := http.NewRequest(method, n.String(), bytes.NewReader(body))
	if err != nil {
		return nil, err
	}

	resp, err := n.client.Do(req)
	switch err := err.(type) {
	default:
		return nil, err

	case nil:
		// check for 307 redirect
		if resp.StatusCode == http.StatusTemporaryRedirect {
			loc, err := resp.Location()
			if err != nil {
				return nil, err
			}

			n.url = strings.Split(loc.String(), "/.json")[0]
			return n.doRequest(method, body)
		}

	case *_url.Error:
		// `http.Client.Do` will return a `url.Error` that wraps a `net.Error`
		// when exceeding it's `Transport`'s `ResponseHeadersTimeout`
		e1, ok := err.Err.(net.Error)
		if ok && e1.Timeout() {
			return nil, apiTimeoutError()
		}

		return nil, err

	case net.Error:
		// `http.Client.Do` will return a `net.Error` directly when Dial times
		// out, or when the Client's RoundTripper otherwise returns an err
		if err.Timeout() {
			return nil, apiTimeoutError()
		}

		return nil, err
	}

	defer resp.Body.Close()
	respBody, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode/200 != 1 {
		apiError := &APIError{}
		err := json.Unmarshal(respBody, &apiError)

		if err != nil {
			return nil, &APIError{
				Type:    "nestapi#json-parse",
				Message: "Unable to parse Nest API JSON",
			}
		}

		return nil, apiError
	}
	return respBody, nil
}

func apiTimeoutError() *APIError {
	return &APIError{
		Type:    "nestapi#timeout",
		Message: "Timeout contacting Nest Server",
	}
}
