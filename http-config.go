//
//
//

package httpstatus

import (
	"context"
	"math/rand"
	"net/http"
	"strings"

	"github.com/ondi/go-log"
)

var (
	LOG_WARN    func(ctx context.Context, format string, args ...any) = log.WarnCtx
	LOG_DEBUG   func(ctx context.Context, format string, args ...any) = log.DebugCtx
	LOG_HEADERS func(r *http.Request) string                          = NewLogHeaders([]string{}).Log
)

type Client interface {
	Do(*http.Request) (*http.Response, error)
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

func (self *Config_t) Header(req *http.Request) (err error) {
	if len(self.Host) > 0 {
		req.Host = self.Host
	}
	for k1, v1 := range self.Headers {
		for _, v2 := range v1 {
			req.Header.Add(k1, v2)
		}
	}
	return
}

func NoHeader(*http.Request) error {
	return nil
}

type LogHeaders_t struct {
	headers []string
}

func NewLogHeaders(headers []string) (self *LogHeaders_t) {
	self = &LogHeaders_t{
		headers: headers,
	}
	return
}

func (self *LogHeaders_t) Log(r *http.Request) string {
	var count int
	var temp string
	var sb strings.Builder
	for _, v := range self.headers {
		if temp = r.Header.Get(v); len(temp) == 0 {
			continue
		}
		if count > 0 {
			sb.WriteString(",")
		}
		sb.WriteString(v)
		sb.WriteString("=")
		sb.WriteString(temp)
		count++
	}
	return sb.String()
}
