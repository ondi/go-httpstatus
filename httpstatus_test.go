//
//
//

package httpstatus

import (
	"context"
	"net/http"
	"testing"

	"gotest.tools/assert"
)

func Test_Status01(t *testing.T) {
	var status HttpStatus_t
	ctx := status.WithClientTrace(context.Background())

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://google.ru", nil)
	assert.NilError(t, err)

	resp, err := http.DefaultClient.Do(req)
	assert.NilError(t, err)

	assert.Assert(t, resp != nil)
	assert.Assert(t, status.GetConnTotal() != 0)
	assert.Assert(t, status.GetTotal() != 0)

	t.Log(status.Report())
}
