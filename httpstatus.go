//
// GetConn()
// DNSStart()
// DNSDone()
// TLSHandshakeStart()
// TLSHandshakeDone()
// ConnectStart()
// ConnectDone()
// GotConn()
// WroteHeaders()
// WroteRequest()
// GotFirstResponseByte()
//

package httpstatus

import (
	"bytes"
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net/http"
	"net/http/httptrace"
	"net/textproto"
	"sort"
	"strconv"
	"sync"
	"time"
)

type Count_t struct {
	Name  string
	Head  time.Time
	Tail  time.Time
	Count int
}

func (self *Count_t) Set(name string, dt time.Time) {
	if self.Count++; self.Count == 1 {
		self.Name = name
		self.Head = dt
	}
	self.Tail = dt
}

type Metrics_t struct {
	mx   sync.Mutex
	data map[string]*Count_t
}

func NewMetrics() *Metrics_t {
	return &Metrics_t{
		data: map[string]*Count_t{},
	}
}

func (self *Metrics_t) Set(name string, dt time.Time) {
	self.mx.Lock()
	defer self.mx.Unlock()
	c := self.data[name]
	if c == nil {
		c = &Count_t{}
		self.data[name] = c
	}
	c.Set(name, dt)
	return
}

func (self *Metrics_t) Get() (out []Count_t) {
	self.mx.Lock()
	defer self.mx.Unlock()
	for _, v := range self.data {
		if v.Count > 0 {
			out = append(out, *v)
		}
	}
	return
}

type Status_t struct {
	Begin   time.Time
	Metrics *Metrics_t

	Hosts  []string
	Errors []error

	Body   bytes.Buffer
	Status int
}

func (self *Status_t) WriteReq(req *http.Request) {
	self.Body.WriteString(req.URL.String())
	self.Body.WriteString(" ")
}

func (self *Status_t) WriteResp(resp *http.Response) {
	self.Status = resp.StatusCode
	self.Body.ReadFrom(resp.Body)
}

func (self *Status_t) WriteRespLimit(resp *http.Response, limit int64) {
	self.Status = resp.StatusCode
	self.Body.ReadFrom(io.LimitReader(resp.Body, limit))
}

func (self *Status_t) WriteStatus(status int, in string) {
	self.Status = status
	self.Body.WriteString(in)
}

func (self *Status_t) String() (res string) {
	res = strconv.FormatInt(int64(self.Status), 10)
	if self.Body.Len() > 0 {
		res += " " + self.Body.String()
	}
	return
}

func (self *Status_t) WithClientTrace(ctx context.Context) context.Context {
	self.Begin = time.Now()
	self.Metrics = NewMetrics()
	return httptrace.WithClientTrace(ctx,
		&httptrace.ClientTrace{
			GetConn:              self.GetConn,
			GotConn:              self.GotConn,
			PutIdleConn:          self.PutIdleConn,
			GotFirstResponseByte: self.GotFirstResponseByte,
			Got100Continue:       self.Got100Continue,
			Got1xxResponse:       self.Got1xxResponse,
			DNSStart:             self.DNSStart,
			DNSDone:              self.DNSDone,
			ConnectStart:         self.ConnectStart,
			ConnectDone:          self.ConnectDone,
			TLSHandshakeStart:    self.TLSHandshakeStart,
			TLSHandshakeDone:     self.TLSHandshakeDone,
			WroteHeaderField:     self.WroteHeaderField,
			WroteHeaders:         self.WroteHeaders,
			Wait100Continue:      self.Wait100Continue,
			WroteRequest:         self.WroteRequest,
		},
	)
}

func ReportMetric(out io.Writer, c Count_t, prev time.Time) time.Time {
	fmt.Fprintf(out, "%-12s: %v %v %v %v\n", c.Name, c.Count, c.Head.Format("15:04:05.000"), c.Tail.Format("15:04:05.000"), c.Tail.Sub(prev))
	return c.Tail
}

func (self *Status_t) Report(out io.Writer) {
	if self.Metrics == nil {
		fmt.Fprintf(out, "NO REPORT\n")
		return
	}
	res := self.Metrics.Get()
	if len(res) == 0 {
		fmt.Fprintf(out, "NO METRICS\n")
		return
	}
	sort.Slice(res, func(i int, j int) bool { return res[i].Head.Before(res[j].Head) })
	fmt.Fprintf(out, "HOSTS : %v\n", self.Hosts)
	fmt.Fprintf(out, "ERRORS: %v\n", self.Errors)
	fmt.Fprintf(out, "TOTAL : %v %v\n", time.Since(self.Begin), res[len(res)-1].Tail.Sub(res[0].Head))
	for _, v := range res {
		ReportMetric(out, v, res[0].Head)
	}
}

// GetConn is called before a connection is created or
// retrieved from an idle pool. The hostPort is the
// "host:port" of the target or proxy. GetConn is called even
// if there's already an idle cached connection available.
func (self *Status_t) GetConn(hostPort string) {
	self.Metrics.Set("GetConn", time.Now())
	self.Hosts = append(self.Hosts, hostPort)
}

// GotConn is called after a successful connection is
// obtained. There is no hook for failure to obtain a
// connection; instead, use the error from
// Transport.RoundTrip.
func (self *Status_t) GotConn(in httptrace.GotConnInfo) {
	self.Metrics.Set("GotConn", time.Now())
}

// PutIdleConn is called when the connection is returned to
// the idle pool. If err is nil, the connection was
// successfully returned to the idle pool. If err is non-nil,
// it describes why not. PutIdleConn is not called if
// connection reuse is disabled via Transport.DisableKeepAlives.
// PutIdleConn is called before the caller's Response.Body.Close
// call returns.
// For HTTP/2, this hook is not currently used.
func (self *Status_t) PutIdleConn(err error) {
	if err != nil {
		self.Errors = append(self.Errors, err)
	}
}

// GotFirstResponseByte is called when the first byte of the response
// headers is available.
func (self *Status_t) GotFirstResponseByte() {
	self.Metrics.Set("ResponseByte", time.Now())
}

// Got100Continue is called if the server replies with a "100
// Continue" response.
func (self *Status_t) Got100Continue() {

}

// Got1xxResponse is called for each 1xx informational response header
// returned before the final non-1xx response. Got1xxResponse is called
// for "100 Continue" responses, even if Got100Continue is also defined.
// If it returns an error, the client request is aborted with that error value.
func (self *Status_t) Got1xxResponse(code int, header textproto.MIMEHeader) (err error) {
	return
}

// DNSStart is called when a DNS lookup begins.
func (self *Status_t) DNSStart(in httptrace.DNSStartInfo) {
	self.Metrics.Set("DNSStart", time.Now())
}

// DNSDone is called when a DNS lookup ends.
func (self *Status_t) DNSDone(in httptrace.DNSDoneInfo) {
	self.Metrics.Set("DNSDone", time.Now())
	if in.Err != nil {
		self.Errors = append(self.Errors, in.Err)
	}
}

// ConnectStart is called when a new connection's Dial begins.
// If net.Dialer.DualStack (IPv6 "Happy Eyeballs") support is
// enabled, this may be called multiple times.
func (self *Status_t) ConnectStart(network, addr string) {
	self.Metrics.Set("ConnectStart", time.Now())
}

// ConnectDone is called when a new connection's Dial
// completes. The provided err indicates whether the
// connection completed successfully.
// If net.Dialer.DualStack ("Happy Eyeballs") support is
// enabled, this may be called multiple times.
func (self *Status_t) ConnectDone(network, addr string, err error) {
	self.Metrics.Set("ConnectDone", time.Now())
	if err != nil {
		self.Errors = append(self.Errors, err)
	}
}

// TLSHandshakeStart is called when the TLS handshake is started. When
// connecting to an HTTPS site via an HTTP proxy, the handshake happens
// after the CONNECT request is processed by the proxy.
func (self *Status_t) TLSHandshakeStart() {
	self.Metrics.Set("TLSHandshakeStart", time.Now())
}

// TLSHandshakeDone is called after the TLS handshake with either the
// successful handshake's connection state, or a non-nil error on handshake
// failure.
func (self *Status_t) TLSHandshakeDone(in tls.ConnectionState, err error) {
	self.Metrics.Set("TLSHandshakeDone", time.Now())
	if err != nil {
		self.Errors = append(self.Errors, err)
	}
}

// WroteHeaderField is called after the Transport has written
// each request header. At the time of this call the values
// might be buffered and not yet written to the network.
func (self *Status_t) WroteHeaderField(key string, value []string) {

}

// WroteHeaders is called after the Transport has written
// all request headers.
func (self *Status_t) WroteHeaders() {

}

// Wait100Continue is called if the Request specified
// "Expect: 100-continue" and the Transport has written the
// request headers but is waiting for "100 Continue" from the
// server before writing the request body.
func (self *Status_t) Wait100Continue() {

}

// WroteRequest is called with the result of writing the
// request and any body. It may be called multiple times
// in the case of retried requests.
func (self *Status_t) WroteRequest(in httptrace.WroteRequestInfo) {
	self.Metrics.Set("WroteRequest", time.Now())
	if in.Err != nil {
		self.Errors = append(self.Errors, in.Err)
	}
}
