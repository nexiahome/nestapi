package nestapi

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/zabawaba99/firetest"
)

func TestSet(t *testing.T) {
	t.Parallel()
	var (
		payload = map[string]interface{}{"foo": "bar"}
		server  = firetest.New()
	)
	server.Start()
	defer server.Close()

	n := New(server.URL)
	err := n.Set(payload)
	assert.NoError(t, err)

	v := server.Get("")
	assert.Equal(t, payload, v)
}
