//
//
//

package httpstatus

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
)

func HttpDo(contexter Contexter, client Client, method string, path string, in []byte, decode func(resp *http.Response) error, header ...func(*http.Request) error) (status Status_t, err error) {
	ctx, cancel := contexter.Ctx()
	defer cancel()
	req, err := http.NewRequestWithContext(
		status.WithClientTrace(ctx),
		method,
		path,
		bytes.NewReader(in),
	)
	if err != nil {
		return
	}

	// some servers refuse multi-headers
	for _, v := range header {
		if err = v(req); err != nil {
			return
		}
	}

	resp, err := client.Do(req)
	if err != nil {
		if errors.Is(err, context.Canceled) == false {
			status.Report(&status.Body)
			if contexter.Options().NoLogs == false {
				LOG_WARN(ctx, "HTTP_REQUEST false status=%s, headers=%v, method=%v, url=%v, err=%v", status.StringFull(), LOG_HEADERS(req), method, req.URL.String(), err)
			}
		}
		return
	}
	defer resp.Body.Close()

	err = decode(resp)
	status.StatusCode = resp.StatusCode
	status.URL.WriteString(req.URL.String())

	if err != nil {
		status.StatusCode = -status.StatusCode
		status.Body.WriteString(err.Error())
		if contexter.Options().NoLogs == false {
			LOG_WARN(ctx, "HTTP_REQUEST false status=%v, headers=%v, method=%v", status.StringFull(), LOG_HEADERS(req), method)
		}
		return status, nil
	}
	if Ok(resp.StatusCode) == false {
		status.Body.ReadFrom(resp.Body)
		if contexter.Options().NoLogs == false {
			LOG_WARN(ctx, "HTTP_REQUEST false status=%v, headers=%v, method=%v", status.StringFull(), LOG_HEADERS(req), method)
		}
		return
	}

	LOG_DEBUG(ctx, "HTTP_REQUEST %v status=%v, headers=%v, method=%v, err=%v", Ok(status.StatusCode), status.StringFull(), LOG_HEADERS(req), method, err)

	return
}

func HttpRequest(context Contexter, client Client, method string, cfg Config_t, path string, in []byte, decode func(*http.Response) error, header func(*http.Request) error) (status Status_t, err error) {
	for _, v := range cfg.Urls() {
		status, err = HttpDo(context, client, method, v+path, in, decode, cfg.Header, header)
		if err == nil {
			break
		}
	}
	return
}

func Skip(resp *http.Response) error {
	return nil
}

func Decode(out any) func(resp *http.Response) (err error) {
	return func(resp *http.Response) (err error) {
		if OkContent(resp.StatusCode) {
			err = json.NewDecoder(resp.Body).Decode(out)
		}
		return
	}
}

func Decode2(out1 any, out2 any) func(resp *http.Response) (err error) {
	return func(resp *http.Response) (err error) {
		switch {
		case OkContent(resp.StatusCode):
			err = json.NewDecoder(resp.Body).Decode(out1)
		case OkNoContent(resp.StatusCode):
			// ok
		case resp.StatusCode == http.StatusBadRequest:
			err = json.NewDecoder(resp.Body).Decode(out2)
		}
		return
	}
}

func Copy(w http.ResponseWriter) func(resp *http.Response) (err error) {
	return func(resp *http.Response) (err error) {
		w.WriteHeader(resp.StatusCode)
		_, err = io.Copy(w, resp.Body)
		return
	}
}

func CopyIO(out io.Writer) func(resp *http.Response) (err error) {
	return func(resp *http.Response) (err error) {
		_, err = io.Copy(out, resp.Body)
		return
	}
}
