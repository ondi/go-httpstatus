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
	"time"

	"github.com/ondi/go-log"
)

type Config_t struct {
	In      []string    `yaml:"Urls"`
	Headers http.Header `yaml:"Headers"`
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

func (self *Config_t) Header(in http.Header) {
	for k1, v1 := range self.Headers {
		for _, v2 := range v1 {
			in.Add(k1, v2)
		}
	}
}

func NoHeader(http.Header) {}

func CopyHeader(to http.Header, from ...http.Header) {
	for _, v1 := range from {
		for k2, v2 := range v1 {
			for _, v3 := range v2 {
				to.Add(k2, v3)
			}
		}
	}
}

func CopyHeaderKey(key string, to http.Header, from ...http.Header) {
	for _, v1 := range from {
		for _, v2 := range v1.Values(key) {
			to.Add(key, v2)
		}
	}
}

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

func HttpDo(contexter Contexter, client Client, method string, path string, in []byte, decode func(resp *http.Response) error, header ...func(http.Header)) (status Status_t, err error) {
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
		v(req.Header)
	}
	resp, err := client.Do(req)
	if err != nil {
		status.URL.WriteString(req.URL.String())
		if errors.Is(err, context.Canceled) == false {
			status.Report(&status.Body)
			log.WarnCtx(ctx, "HTTP_REQUEST: false %v %s", err, status.Body.Bytes())
		}
		return
	}
	defer resp.Body.Close()

	status.StatusCode = resp.StatusCode
	err = decode(resp)

	if err != nil {
		status.StatusCode = -status.StatusCode
		status.URL.WriteString(req.URL.String())
		status.Body.WriteString(err.Error())
		log.WarnCtx(ctx, "HTTP_REQUEST: false %v %v", method, status.String())
		return status, nil
	}
	if Ok(resp.StatusCode) == false {
		status.URL.WriteString(req.URL.String())
		status.Body.ReadFrom(resp.Body)
		log.WarnCtx(ctx, "HTTP_REQUEST: false %v %v", method, status.String())
		return
	}

	log.DebugCtx(ctx, "HTTP_REQUEST: %v %v %v %v %v", Ok(status.StatusCode), err, method, status.StatusCode, req.URL)

	return
}

// some http servers refuse multiple headers with same name
func HttpRequest(context Contexter, client Client, method string, cfg Config_t, path string, in []byte, decode func(resp *http.Response) error, header func(http.Header)) (status Status_t, err error) {
	for _, v := range cfg.Urls() {
		status, err = HttpDo(context, client, method, v+path, in, decode, cfg.Header, header)
		if err == nil {
			break
		}
	}
	return
}
