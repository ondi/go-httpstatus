//
// GetConn()
// DNSStart()
// DNSDone()
// ConnectStart()
// ConnectDone()
// TLSHandshakeStart()
// TLSHandshakeDone()
// GotConn()
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
	"strconv"
	"time"
)

type Data_t struct {
	Begin time.Time
	Count int
	Sum   time.Duration
}

type HttpStatus_t struct {
	hosts []string

	got_conn time.Time //  GetConn() -> DNS{Start,Done}() + Connect{Start,Done}() -> TLSHandshake{Start,Done}()-> GotConn()

	get_conn      Data_t
	dns_start     Data_t
	connect_start Data_t
	tls_start     Data_t
	request       Data_t
	response      Data_t

	err error

	body bytes.Buffer
	code int
}

func (self *HttpStatus_t) Read(resp *http.Response) {
	self.code = resp.StatusCode
	self.body.ReadFrom(io.LimitReader(resp.Body, 1024))
}

func (self *HttpStatus_t) WriteString(code int, in string) {
	self.code = code
	self.body.WriteString(in)
}

func (self *HttpStatus_t) Code() int {
	return self.code
}

func (self *HttpStatus_t) String() (res string) {
	res = strconv.FormatInt(int64(self.code), 10)
	if self.body.Len() > 0 {
		res += " " + self.body.String()
	}
	return
}

func (self *HttpStatus_t) WithClientTrace(ctx context.Context) context.Context {
	trace := httptrace.ClientTrace{
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
	}
	return httptrace.WithClientTrace(ctx, &trace)
}

func (self *HttpStatus_t) GetTotal() time.Duration {
	return self.get_conn.Sum + self.request.Sum + self.response.Sum
}

func (self *HttpStatus_t) GetConnTotal() time.Duration {
	return self.get_conn.Sum
}

func (self *HttpStatus_t) Report() string {
	return fmt.Sprintf(`Hosts: %v
GetConn: %v %v
  Dns: %v %v
  Connect: %v %v
  Tls: %v %v
Request: %v %v
Response: %v %v
Total: %v
Error: %v`,
		self.hosts,
		self.get_conn.Count, self.get_conn.Sum,
		self.dns_start.Count, self.dns_start.Sum,
		self.connect_start.Count, self.connect_start.Sum,
		self.tls_start.Count, self.tls_start.Sum,
		self.request.Count, self.request.Sum,
		self.response.Count, self.response.Sum,
		self.GetTotal(),
		self.err,
	)
}

// GetConn is called before a connection is created or
// retrieved from an idle pool. The hostPort is the
// "host:port" of the target or proxy. GetConn is called even
// if there's already an idle cached connection available.
func (self *HttpStatus_t) GetConn(hostPort string) {
	self.get_conn.Count++
	self.get_conn.Begin = time.Now()
	self.hosts = append(self.hosts, hostPort)
}

// GotConn is called after a successful connection is
// obtained. There is no hook for failure to obtain a
// connection; instead, use the error from
// Transport.RoundTrip.
func (self *HttpStatus_t) GotConn(in httptrace.GotConnInfo) {
	self.got_conn = time.Now()
	self.get_conn.Sum += self.got_conn.Sub(self.get_conn.Begin)
}

// PutIdleConn is called when the connection is returned to
// the idle pool. If err is nil, the connection was
// successfully returned to the idle pool. If err is non-nil,
// it describes why not. PutIdleConn is not called if
// connection reuse is disabled via Transport.DisableKeepAlives.
// PutIdleConn is called before the caller's Response.Body.Close
// call returns.
// For HTTP/2, this hook is not currently used.
func (self *HttpStatus_t) PutIdleConn(err error) {
	if err != nil {
		self.err = err
	}
}

// GotFirstResponseByte is called when the first byte of the response
// headers is available.
func (self *HttpStatus_t) GotFirstResponseByte() {
	self.response.Count++
	self.response.Begin = time.Now()
	self.response.Sum += self.response.Begin.Sub(self.request.Begin)
}

// Got100Continue is called if the server replies with a "100
// Continue" response.
func (self *HttpStatus_t) Got100Continue() {

}

// Got1xxResponse is called for each 1xx informational response header
// returned before the final non-1xx response. Got1xxResponse is called
// for "100 Continue" responses, even if Got100Continue is also defined.
// If it returns an error, the client request is aborted with that error value.
func (self *HttpStatus_t) Got1xxResponse(code int, header textproto.MIMEHeader) (err error) {
	return
}

// DNSStart is called when a DNS lookup begins.
func (self *HttpStatus_t) DNSStart(in httptrace.DNSStartInfo) {
	self.dns_start.Count++
	self.dns_start.Begin = time.Now()
}

// DNSDone is called when a DNS lookup ends.
func (self *HttpStatus_t) DNSDone(in httptrace.DNSDoneInfo) {
	self.dns_start.Sum += time.Since(self.dns_start.Begin)
	if in.Err != nil {
		self.err = in.Err
	}
}

// ConnectStart is called when a new connection's Dial begins.
// If net.Dialer.DualStack (IPv6 "Happy Eyeballs") support is
// enabled, this may be called multiple times.
func (self *HttpStatus_t) ConnectStart(network, addr string) {
	self.connect_start.Count++
	self.connect_start.Begin = time.Now()
}

// ConnectDone is called when a new connection's Dial
// completes. The provided err indicates whether the
// connection completed successfully.
// If net.Dialer.DualStack ("Happy Eyeballs") support is
// enabled, this may be called multiple times.
func (self *HttpStatus_t) ConnectDone(network, addr string, err error) {
	self.connect_start.Sum += time.Since(self.connect_start.Begin)
	if err != nil {
		self.err = err
	}
}

// TLSHandshakeStart is called when the TLS handshake is started. When
// connecting to an HTTPS site via an HTTP proxy, the handshake happens
// after the CONNECT request is processed by the proxy.
func (self *HttpStatus_t) TLSHandshakeStart() {
	self.tls_start.Count++
	self.tls_start.Begin = time.Now()
}

// TLSHandshakeDone is called after the TLS handshake with either the
// successful handshake's connection state, or a non-nil error on handshake
// failure.
func (self *HttpStatus_t) TLSHandshakeDone(in tls.ConnectionState, err error) {
	self.tls_start.Sum += time.Since(self.tls_start.Begin)
	if err != nil {
		self.err = err
	}
}

// WroteHeaderField is called after the Transport has written
// each request header. At the time of this call the values
// might be buffered and not yet written to the network.
func (self *HttpStatus_t) WroteHeaderField(key string, value []string) {

}

// WroteHeaders is called after the Transport has written
// all request headers.
func (self *HttpStatus_t) WroteHeaders() {

}

// Wait100Continue is called if the Request specified
// "Expect: 100-continue" and the Transport has written the
// request headers but is waiting for "100 Continue" from the
// server before writing the request body.
func (self *HttpStatus_t) Wait100Continue() {

}

// WroteRequest is called with the result of writing the
// request and any body. It may be called multiple times
// in the case of retried requests.
func (self *HttpStatus_t) WroteRequest(in httptrace.WroteRequestInfo) {
	self.request.Count++
	self.request.Begin = time.Now()
	self.request.Sum += self.request.Begin.Sub(self.got_conn)
	if in.Err != nil {
		self.err = in.Err
	}
}
