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

var STATUS_OK = map[int]bool{
	http.StatusOK:       true,
	http.StatusCreated:  true,
	http.StatusAccepted: true,
}

var STATUS_OK_NO_CONTENT = map[int]bool{
	http.StatusNoContent: true,
}

type Count_t[T any] struct {
	Head  T
	Tail  T
	Name  string
	Count int
}

func (self *Count_t[T]) Set(name string, in T) {
	if self.Count++; self.Count == 1 {
		self.Name = name
		self.Head = in
	}
	self.Tail = in
}

type Trace_t struct {
	mx     sync.Mutex
	begin  time.Time
	times  map[string]*Count_t[time.Time]
	hosts  map[string]*Count_t[string]
	errors map[string]*Count_t[error]
}

func NewTrace(dt time.Time) *Trace_t {
	return &Trace_t{
		begin:  dt,
		times:  map[string]*Count_t[time.Time]{},
		hosts:  map[string]*Count_t[string]{},
		errors: map[string]*Count_t[error]{},
	}
}

func (self *Trace_t) SetTime(name string, in time.Time) {
	self.mx.Lock()
	defer self.mx.Unlock()
	c := self.times[name]
	if c == nil {
		c = &Count_t[time.Time]{}
		self.times[name] = c
	}
	c.Set(name, in)
}

func (self *Trace_t) SetHost(name string, in string) {
	self.mx.Lock()
	defer self.mx.Unlock()
	c := self.hosts[name]
	if c == nil {
		c = &Count_t[string]{}
		self.hosts[name] = c
	}
	c.Set(name, in)
}

func (self *Trace_t) SetError(name string, in error) {
	self.mx.Lock()
	defer self.mx.Unlock()
	c := self.errors[name]
	if c == nil {
		c = &Count_t[error]{}
		self.errors[name] = c
	}
	c.Set(name, in)
}

func (self *Trace_t) Report(out io.Writer) {
	self.mx.Lock()
	defer self.mx.Unlock()
	fmt.Fprintf(out, "TRACE : times=%v, hosts=%v, errors=%v\n", len(self.times), len(self.hosts), len(self.errors))
	for k, v := range self.hosts {
		if v.Count == 1 {
			fmt.Fprintf(out, "HOSTS : %v: %v, %v\n", k, v.Count, v.Tail)
		} else {
			fmt.Fprintf(out, "HOSTS : %v: %v, %v, %v\n", k, v.Count, v.Head, v.Tail)
		}
	}
	for k, v := range self.errors {
		if v.Count == 1 {
			fmt.Fprintf(out, "ERRORS: %v: %v, %v\n", k, v.Count, v.Tail)
		} else {
			fmt.Fprintf(out, "ERRORS: %v: %v, %v, %v\n", k, v.Count, v.Head, v.Tail)
		}
	}
	if len(self.times) == 0 {
		return
	}
	var times []*Count_t[time.Time]
	for _, v := range self.times {
		times = append(times, v)
	}
	sort.Slice(times, func(i int, j int) bool { return times[i].Head.Before(times[j].Head) })
	fmt.Fprintf(out, "TOTAL : %v %v\n", time.Since(self.begin), times[len(times)-1].Tail.Sub(times[0].Head))
	for _, v := range times {
		ReportMetric(out, v, times[0].Head)
	}
	return
}

func ReportMetric(out io.Writer, c *Count_t[time.Time], prev time.Time) time.Time {
	if c.Count == 1 {
		fmt.Fprintf(out, "%-16s: %v %v %v\n", c.Name, c.Count, c.Tail.Format("15:04:05.000"), c.Tail.Sub(prev))
	} else {
		fmt.Fprintf(out, "%-16s: %v %v %v %v\n", c.Name, c.Count, c.Head.Format("15:04:05.000"), c.Tail.Format("15:04:05.000"), c.Tail.Sub(prev))
	}
	return c.Tail
}

type Status_t struct {
	trace      *Trace_t
	Body       bytes.Buffer
	StatusCode int
}

func (self *Status_t) StatusOk() (ok bool) {
	return STATUS_OK[self.StatusCode] || STATUS_OK_NO_CONTENT[self.StatusCode]
}

func (self *Status_t) String() (res string) {
	res = strconv.FormatInt(int64(self.StatusCode), 10)
	if self.Body.Len() > 0 {
		res += " " + self.Body.String()
	}
	return
}

func (self *Status_t) WithClientTrace(ctx context.Context) context.Context {
	self.trace = NewTrace(time.Now())
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

func (self *Status_t) Report(out io.Writer) {
	self.trace.Report(out)
}

// GetConn is called before a connection is created or
// retrieved from an idle pool. The hostPort is the
// "host:port" of the target or proxy. GetConn is called even
// if there's already an idle cached connection available.
func (self *Status_t) GetConn(hostPort string) {
	self.trace.SetTime("GetConn", time.Now())
	self.trace.SetHost("GetConn", hostPort)
}

// GotConn is called after a successful connection is
// obtained. There is no hook for failure to obtain a
// connection; instead, use the error from
// Transport.RoundTrip.
func (self *Status_t) GotConn(in httptrace.GotConnInfo) {
	self.trace.SetTime("GotConn", time.Now())
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
	// self.trace.SetTime("PutIdleConn", time.Now())
	if err != nil {
		self.trace.SetError("PutIdleConn", err)
	}
}

// GotFirstResponseByte is called when the first byte of the response
// headers is available.
func (self *Status_t) GotFirstResponseByte() {
	self.trace.SetTime("ResponseByte", time.Now())
}

// Got100Continue is called if the server replies with a "100
// Continue" response.
func (self *Status_t) Got100Continue() {
	self.trace.SetTime("100Continue", time.Now())
}

// Got1xxResponse is called for each 1xx informational response header
// returned before the final non-1xx response. Got1xxResponse is called
// for "100 Continue" responses, even if Got100Continue is also defined.
// If it returns an error, the client request is aborted with that error value.
func (self *Status_t) Got1xxResponse(code int, header textproto.MIMEHeader) (err error) {
	self.trace.SetTime("1xxResponse", time.Now())
	return
}

// DNSStart is called when a DNS lookup begins.
func (self *Status_t) DNSStart(in httptrace.DNSStartInfo) {
	self.trace.SetTime("DNSStart", time.Now())
}

// DNSDone is called when a DNS lookup ends.
func (self *Status_t) DNSDone(in httptrace.DNSDoneInfo) {
	self.trace.SetTime("DNSDone", time.Now())
	if in.Err != nil {
		self.trace.SetError("DNSDone", in.Err)
	}
}

// ConnectStart is called when a new connection's Dial begins.
// If net.Dialer.DualStack (IPv6 "Happy Eyeballs") support is
// enabled, this may be called multiple times.
func (self *Status_t) ConnectStart(network, addr string) {
	self.trace.SetTime("ConnectStart", time.Now())
}

// ConnectDone is called when a new connection's Dial
// completes. The provided err indicates whether the
// connection completed successfully.
// If net.Dialer.DualStack ("Happy Eyeballs") support is
// enabled, this may be called multiple times.
func (self *Status_t) ConnectDone(network, addr string, err error) {
	self.trace.SetTime("ConnectDone", time.Now())
	if err != nil {
		self.trace.SetError("ConnectDone", err)
	}
}

// TLSHandshakeStart is called when the TLS handshake is started. When
// connecting to an HTTPS site via an HTTP proxy, the handshake happens
// after the CONNECT request is processed by the proxy.
func (self *Status_t) TLSHandshakeStart() {
	self.trace.SetTime("TLSStart", time.Now())
}

// TLSHandshakeDone is called after the TLS handshake with either the
// successful handshake's connection state, or a non-nil error on handshake
// failure.
func (self *Status_t) TLSHandshakeDone(in tls.ConnectionState, err error) {
	self.trace.SetTime("TLSDone", time.Now())
	if err != nil {
		self.trace.SetError("TLSDone", err)
	}
}

// WroteHeaderField is called after the Transport has written
// each request header. At the time of this call the values
// might be buffered and not yet written to the network.
func (self *Status_t) WroteHeaderField(key string, value []string) {
	self.trace.SetTime("WroteHeaderField", time.Now())
}

// WroteHeaders is called after the Transport has written
// all request headers.
func (self *Status_t) WroteHeaders() {
	self.trace.SetTime("WroteHeaders", time.Now())
}

// Wait100Continue is called if the Request specified
// "Expect: 100-continue" and the Transport has written the
// request headers but is waiting for "100 Continue" from the
// server before writing the request body.
func (self *Status_t) Wait100Continue() {
	self.trace.SetTime("Wait100Continue", time.Now())
}

// WroteRequest is called with the result of writing the
// request and any body. It may be called multiple times
// in the case of retried requests.
func (self *Status_t) WroteRequest(in httptrace.WroteRequestInfo) {
	self.trace.SetTime("WroteRequest", time.Now())
	if in.Err != nil {
		self.trace.SetError("WroteRequest", in.Err)
	}
}
