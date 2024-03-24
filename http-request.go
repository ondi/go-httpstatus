//
//
//

package httpstatus

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"math/rand"
	"net/http"
	"time"
)

type Http_t struct {
	In      []string    `yaml:"Urls"`
	Headers http.Header `yaml:"Headers"`
	Body    string      `yaml:"Body"`
}

func (self *Http_t) Len() int {
	return len(self.In)
}

func (self *Http_t) Urls() (res []string) {
	res = make([]string, len(self.In))
	for i, v := range rand.Perm(len(self.In)) {
		res[i] = self.In[v]
	}
	return
}

func (self *Http_t) Header(in http.Header) {
	for k1, v1 := range self.Headers {
		for _, v2 := range v1 {
			in.Add(k1, v2)
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

func HttpDo(context Contexter, client Client, method string, URL string, in []byte, decode func(resp *http.Response) error, headers ...func(http.Header)) (status Status_t, err error) {
	ctx, cancel := context.Get()
	defer cancel()
	req, err := http.NewRequestWithContext(
		status.WithClientTrace(ctx),
		method,
		URL,
		bytes.NewReader(in),
	)
	if err != nil {
		return
	}
	for _, v := range headers {
		v(req.Header)
	}
	resp, err := client.Do(req)
	if err != nil {
		status.Report(&status.Body)
		return
	}
	defer resp.Body.Close()

	status.StatusCode = resp.StatusCode
	err = decode(resp)

	if err != nil {
		status.StatusCode = -status.StatusCode
		status.Body.WriteString(req.URL.String())
		status.Body.WriteString(" ")
		status.Body.WriteString(err.Error())
		return status, nil
	}
	if Ok(resp.StatusCode) == false {
		status.Body.WriteString(req.URL.String())
		status.Body.WriteString(" ")
		status.Body.ReadFrom(resp.Body)
		return
	}

	return
}

// some http servers refuse multiple headers with same name
func HttpRequest(context Contexter, client Client, method string, cfg Http_t, path string, in []byte, decode func(resp *http.Response) error, headers ...func(http.Header)) (status Status_t, err error) {
	for _, v := range cfg.Urls() {
		status, err = HttpDo(context, client, method, v+path, in, decode, append([]func(http.Header){cfg.Header}, headers...)...)
		if err == nil {
			break
		}
	}
	return
}
