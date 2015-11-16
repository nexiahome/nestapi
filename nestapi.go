/*
Package nestapi is a REST client for NestAPI (https://firebase.com).
*/
package nestapi

import (
	"bytes"
	"errors"
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
// an ErrTimeout error
var TimeoutDuration = 30 * time.Second

var defaultRedirectLimit = 30

// ErrTimeout is an error type is that is returned if a request
// exceeds the TimeoutDuration configured
type ErrTimeout struct {
	error
}

// query parameter constants
const (
	authParam    = "auth"
	formatParam  = "format"
	shallowParam = "shallow"
	formatVal    = "export"
)

// NestAPI represents a location in the cloud
type NestAPI struct {
	url    string
	params _url.Values
	client *http.Client

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

// Preserve headers on redirect
// See: https://github.com/golang/go/issues/4800
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

// New creates a new NestAPI reference
func New(url string) *NestAPI {

	var tr *http.Transport
	tr = &http.Transport{
		DisableKeepAlives: true, // https://code.google.com/p/go/issues/detail?id=3514
		Dial: func(network, address string) (net.Conn, error) {
			start := time.Now()
			c, err := net.DialTimeout(network, address, TimeoutDuration)
			tr.ResponseHeaderTimeout = TimeoutDuration - time.Since(start)
			return c, err
		},
	}

	var client *http.Client
	client = &http.Client{
		Transport:     tr,
		CheckRedirect: redirectPreserveHeaders,
	}

	return &NestAPI{
		url:          sanitizeURL(url),
		params:       _url.Values{},
		client:       client,
		stopWatching: make(chan struct{}),
	}
}

// String returns the string representation of the
// NestAPI reference
func (n *NestAPI) String() string {
	return n.url
}

// Child creates a new NestAPI reference for the requested
// child with the same configuration as the parent
func (n *NestAPI) Child(child string) *NestAPI {
	c := &NestAPI{
		url:          n.url + "/" + child,
		params:       _url.Values{},
		client:       n.client,
		stopWatching: make(chan struct{}),
	}

	// making sure to manually copy the map items into a new
	// map to avoid modifying the map reference.
	for k, v := range n.params {
		c.params[k] = v
	}
	return c
}

// Shallow limits the depth of the data returned when calling Value.
// If the data at the location is a JSON primitive (string, number or boolean),
// its value will be returned. If the data is a JSON object, the values
// for each key will be truncated to true.
//
// Reference https://www.firebase.com/docs/rest/api/#section-param-shallow
func (n *NestAPI) Shallow(v bool) {
	if v {
		n.params.Set(shallowParam, "true")
	} else {
		n.params.Del(shallowParam)
	}
}

// IncludePriority determines whether or not to ask NestAPI
// for the values priority. By default, the priority is not returned
//
// Reference https://www.firebase.com/docs/rest/api/#section-param-format
func (n *NestAPI) IncludePriority(v bool) {
	if v {
		n.params.Set(formatParam, formatVal)
	} else {
		n.params.Del(formatParam)
	}
}

func (n *NestAPI) makeRequest(method string, body []byte) (*http.Request, error) {
	path := n.url + "/.json"

	if len(n.params) > 0 {
		path += "?" + n.params.Encode()
	}
	return http.NewRequest(method, path, bytes.NewReader(body))
}

func (n *NestAPI) doRequest(method string, body []byte) ([]byte, error) {
	req, err := n.makeRequest(method, body)
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
			return nil, ErrTimeout{err}
		}

		return nil, err

	case net.Error:
		// `http.Client.Do` will return a `net.Error` directly when Dial times
		// out, or when the Client's RoundTripper otherwise returns an err
		if err.Timeout() {
			return nil, ErrTimeout{err}
		}

		return nil, err
	}

	defer resp.Body.Close()
	respBody, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode/200 != 1 {
		return nil, errors.New(string(respBody))
	}
	return respBody, nil
}
