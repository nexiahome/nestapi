package nestapi

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/zabawaba99/firetest"
)

const authToken = "token"

func TestAuth(t *testing.T) {
	t.Parallel()
	server := firetest.New()
	server.Start()
	defer server.Close()

	server.RequireAuth(true)
	n := New(server.URL)

	n.Auth(server.Secret)
	var v interface{}
	err := n.Set(&v)
	assert.NoError(t, err)
}

func TestUnauth(t *testing.T) {
	t.Parallel()
	server := firetest.New()
	server.Start()
	defer server.Close()

	server.RequireAuth(true)
	n := New(server.URL)

	n.params.Add("auth", server.Secret)
	n.Unauth()
	err := n.Set("")
	assert.Error(t, err)
}
