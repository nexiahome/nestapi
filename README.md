# Nestapi
---
[![Build Status](https://travis-ci.org/jgeiger/nestapi.svg?branch=master)](https://travis-ci.org/jgeiger/nestapi) [![Coverage Status](https://coveralls.io/repos/jgeiger/nestapi/badge.svg)](https://coveralls.io/r/jgeiger/nestapi)
---

A Nest API client written in Go

Almost all of this code was copied from [firego](https://github.com/CloudCom/firego)
I removed the parts that aren't needed for the Nest API, and changed the watch to return the full JSON string instead of interfaces. We know what Nest will provide so we can
just Unmarshall the JSON into a struct.

##### Under Development
The API may or may not change radically within the next upcoming weeks.

## Installation

```bash
go get -u github.com/jgeiger/nestapi
```

## Usage

Import nestapi

```go
import "github.com/jgeiger/nestapi"
```

Create a new nestapi reference

```go
f := nestapi.New("https://api.home.nest.com")
```

### Request Timeouts

By default, the `NestAPI` reference will timeout after 30 seconds of trying
to reach the Nest API server. You can configure this value by setting the global
timeout duration

```go
nestapi.TimeoutDuration = time.Minute
```

### Auth Tokens

```go
f.Auth("some-token-that-was-created-for-me")
f.Unauth()
```

### Set Value

```go
v := map[string]string{"foo":"bar"}
if err := f.Set(v); err != nil {
  log.Fatal(err)
}
```

### Watch a Node

```go
notifications := make(chan nestapi.Event)
if err := f.Watch(notifications); err != nil {
	log.Fatal(err)
}

defer f.StopWatching()
for event := range notifications {
	fmt.Printf("Event %#v\n", event)
}
fmt.Printf("Notifications have stopped")
```

Check the [GoDocs](http://godoc.org/github.com/jgeiger/nestapi) or
[Nest API Documentation](https://developer.nest.com/documentation/api-reference) for more details

## Running Tests

In order to run the tests you need to `go get`:

* `github.com/stretchr/testify/require`
* `github.com/stretchr/testify/assert`

## Issues Management

Feel free to open an issue if you come across any bugs or
if you'd like to request a new feature.

## Contributing

1. Fork it
2. Create your feature branch (`git checkout -b new-feature`)
3. Commit your changes (`git commit -am 'Some cool reflection'`)
4. Push to the branch (`git push origin new-feature`)
5. Create new Pull Request
