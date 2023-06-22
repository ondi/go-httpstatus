//
//
//

package httpstatus

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"time"

	"gotest.tools/assert"
)

func Test_Status01(t *testing.T) {
	var status Status_t
	ctx := status.WithClientTrace(context.Background())
	ctx, cancel := context.WithTimeout(ctx, time.Second)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://google.ru", nil)
	assert.NilError(t, err)

	resp, err := http.DefaultClient.Do(req)
	assert.NilError(t, err)

	cancel()

	assert.Assert(t, resp != nil)
	assert.Assert(t, status.GetTotal() != 0)

	t.Logf("%+v\n", status.GetTotal())

	var sb strings.Builder
	status.Report(&sb)
	t.Logf("%s\n", sb.String())
}
