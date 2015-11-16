package nestapi_test

import (
	"log"
	"time"

	"github.com/jgeiger/nestapi"
)

func ExampleNestAPI_Auth() {
	n := nestapi.New("https://someapp.firebaseio.com")
	n.Auth("my-token")
}

func ExampleNestAPI_Child() {
	n := nestapi.New("https://someapp.firebaseio.com")
	childNestAPI := n.Child("some/child/path")

	log.Printf("My new ref %s\n", childNestAPI)
}

func ExampleNestAPI_Set() {
	n := nestapi.New("https://someapp.firebaseio.com")

	v := map[string]interface{}{
		"foo": "bar",
		"bar": 1,
		"bez": []string{"hello", "world"},
	}
	if err := n.Set(v); err != nil {
		log.Fatal(err)
	}
}

func ExampleNestAPI_Watch() {
	n := nestapi.New("https://someapp.firebaseio.com/some/value")
	notifications := make(chan nestapi.Event)
	if err := n.Watch(notifications); err != nil {
		log.Fatal(err)
	}

	for event := range notifications {
		log.Println("Event Received")
		log.Printf("Type: %s\n", event.Type)
		log.Printf("Data: %v\n", event.Data)
		if event.Type == nestapi.EventTypeError {
			log.Print("Error occurred, loop ending")
		}
	}
}

func ExampleNestAPI_StopWatching() {
	n := nestapi.New("https://someapp.firebaseio.com/some/value")
	notifications := make(chan nestapi.Event)
	if err := n.Watch(notifications); err != nil {
		log.Fatal(err)
	}

	go func() {
		for _ = range notifications {
		}
		log.Println("Channel closed")
	}()
	time.Sleep(10 * time.Millisecond) // let go routine start

	n.StopWatching()
}
