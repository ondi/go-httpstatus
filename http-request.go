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
	"math/rand"
	"net/http"
	"strings"
	"time"

	"github.com/ondi/go-log"
)

var LOG_HEADERS = func(r *http.Request) string {
	var count int
	var sb strings.Builder
	sb.WriteString("{")
	for _, v := range []string{} {
		if count > 0 {
			sb.WriteString(",")
		}
		sb.WriteString(v)
		sb.WriteString("=")
		sb.WriteString(r.Header.Get(v))
		count++
	}
	sb.WriteString("}")
	return sb.String()
}

type Config_t struct {
	In      []string    `yaml:"Urls"`
	Headers http.Header `yaml:"Headers"`
	Host    string      `yaml:"Host"`
	Body    string      `yaml:"Body"`
}

func (self *Config_t) Len() int {
	return len(self.In)
}

func (self *Config_t) Urls() (res []string) {
	res = make([]string, len(self.In))
	for i, v := range rand.Perm(len(self.In)) {
		res[i] = self.In[v]
	}
	return
}

func (self *Config_t) Header(req *http.Request) {
	if len(self.Host) > 0 {
		req.Host = self.Host
	}
	for k1, v1 := range self.Headers {
		for _, v2 := range v1 {
			req.Header.Add(k1, v2)
		}
	}
}

func NoHeader(*http.Request) {}

type Client interface {
	Do(*http.Request) (*http.Response, error)
}

type Contexter interface {
	Get() (context.Context, context.CancelFunc)
}

type Pass struct {
	context.Context
}

func (self Pass) Get() (context.Context, context.CancelFunc) {
	return self.Context, func() {}
}

type Timeout struct {
	ctx     context.Context
	timeout time.Duration
}

func (self Timeout) Get() (context.Context, context.CancelFunc) {
	return context.WithTimeout(self.ctx, self.timeout)
}

func Context(ctx context.Context) Contexter {
	return Pass{Context: ctx}
}

func ContextTimeout(ctx context.Context, timeout time.Duration) Contexter {
	return Timeout{ctx: ctx, timeout: timeout}
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

func HttpDo(contexter Contexter, client Client, method string, path string, in []byte, decode func(resp *http.Response) error, header ...func(*http.Request)) (status Status_t, err error) {
	ctx, cancel := contexter.Get()
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

	for _, v := range header {
		v(req)
	}

	// remove later
	for k, v := range req.Header {
		if len(v) > 1 {
			log.WarnCtx(ctx, "HTTP_REQUEST: HEADER LENGTH %v=%v, url=%v", k, len(v), req.URL.String())
		}
	}

	resp, err := client.Do(req)
	if err != nil {
		if errors.Is(err, context.Canceled) == false {
			status.Report(&status.Body)
			log.WarnCtx(ctx, "HTTP_REQUEST: false method=%v, status=%s, headers=%v, url=%v, err=%v", method, status.String(), LOG_HEADERS(req), req.URL.String(), err)
		}
		return
	}
	defer resp.Body.Close()

	status.URL.WriteString(req.URL.String())
	status.StatusCode = resp.StatusCode
	err = decode(resp)

	if err != nil {
		status.StatusCode = -status.StatusCode
		status.Body.WriteString(err.Error())
		log.WarnCtx(ctx, "HTTP_REQUEST: false method=%v, status=%v, headers=%v", method, status.String(), LOG_HEADERS(req))
		return status, nil
	}
	if Ok(resp.StatusCode) == false {
		status.Body.ReadFrom(resp.Body)
		log.WarnCtx(ctx, "HTTP_REQUEST: false method=%v, status=%v, headers=%v", method, status.String(), LOG_HEADERS(req))
		return
	}

	log.DebugCtx(ctx, "HTTP_REQUEST: %v method=%v, status=%v, headers=%v, err=%v", Ok(status.StatusCode), method, status.String(), LOG_HEADERS(req), err)

	return
}

// some http servers refuse multiple headers with same name
func HttpRequest(context Contexter, client Client, method string, cfg Config_t, path string, in []byte, decode func(resp *http.Response) error, header func(*http.Request)) (status Status_t, err error) {
	for _, v := range cfg.Urls() {
		status, err = HttpDo(context, client, method, v+path, in, decode, cfg.Header, header)
		if err == nil {
			break
		}
	}
	return
}
