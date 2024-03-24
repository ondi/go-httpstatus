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
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://google.ru", nil)
	assert.NilError(t, err)

	_, err = http.DefaultClient.Do(req)
	t.Logf("CONNECT=%v", err)

	cancel()

	var sb strings.Builder
	status.Report(&sb)
	t.Logf("%s\n", sb.String())
}
