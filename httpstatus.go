//
// GetConn()
// ConnectStart()
// DNS{Start,Done}()
// TLSHandshake{Start,Done}()
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
	"strconv"
	"strings"
	"time"
)

type Begin_t struct {
	Begin time.Time
	Count int
}

type End_t struct {
	End   time.Time
	Count int
}

type Status_t struct {
	Hosts         []string
	XGetConn      Begin_t
	XConnectStart Begin_t
	XDnsStart     Begin_t
	XDnsDone      End_t
	XTlsStart     Begin_t
	XTlsDone      End_t
	XConnectDone  End_t
	XGotConn      End_t
	XRequest      End_t
	XResponse     End_t
	Err           error

	body bytes.Buffer
	code int
}

func (self *Status_t) Read(resp *http.Response) {
	self.SetCode(resp.StatusCode)
	self.ReadFrom(resp.Body)
}

func (self *Status_t) ReadLimit(resp *http.Response, limit int64) {
	self.SetCode(resp.StatusCode)
	self.ReadFromLimit(resp.Body, limit)
}

func (self *Status_t) WriteStatus(code int, in string) {
	self.SetCode(code)
	self.WriteString(in)
}

func (self *Status_t) WriteStatusLess(code_less int, code int, in string) {
	if self.code < code_less {
		self.SetCode(code)
	}
	self.WriteString(in)
}

func (self *Status_t) SetCode(in int) {
	self.code = in
}

func (self *Status_t) WriteString(in string) {
	self.body.WriteString(in)
}

func (self *Status_t) ReadFrom(in io.Reader) (int64, error) {
	return self.body.ReadFrom(in)
}

func (self *Status_t) ReadFromLimit(in io.Reader, limit int64) (int64, error) {
	return self.body.ReadFrom(io.LimitReader(in, limit))
}

func (self *Status_t) Code() int {
	return self.code
}

func (self *Status_t) String() (res string) {
	res = strconv.FormatInt(int64(self.code), 10)
	if self.body.Len() > 0 {
		res += " " + self.body.String()
	}
	return
}

func (self *Status_t) Bytes() []byte {
	return self.body.Bytes()
}

func (self *Status_t) WithClientTrace(ctx context.Context) context.Context {
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

func (self *Status_t) GetTotal() time.Duration {
	return self.XResponse.End.Sub(self.XGetConn.Begin)
}

func (self *Status_t) Report(out *strings.Builder) {
	fmt.Fprintf(out, `
XGetConn     : %v %v
XConnectStart: %v %v
XDnsStart    : %v %v
XDnsDone     : %v %v
XTlsStart    : %v %v
XTlsDone     : %v %v
XConnectDone : %v %v
XGotConn     : %v %v
XRequest     : %v %v
XResponse    : %v %v
`,
		self.XGetConn.Count, self.XGetConn.Begin.Format("2006-01-02 15:04:05.000000"),
		self.XConnectStart.Count, self.XConnectStart.Begin.Format("2006-01-02 15:04:05.000000"),
		self.XDnsStart.Count, self.XDnsStart.Begin.Format("2006-01-02 15:04:05.000000"),
		self.XDnsDone.Count, self.XDnsDone.End.Format("2006-01-02 15:04:05.000000"),
		self.XTlsStart.Count, self.XTlsStart.Begin.Format("2006-01-02 15:04:05.000000"),
		self.XTlsDone.Count, self.XTlsDone.End.Format("2006-01-02 15:04:05.000000"),
		self.XConnectDone.Count, self.XConnectDone.End.Format("2006-01-02 15:04:05.000000"),
		self.XGotConn.Count, self.XGotConn.End.Format("2006-01-02 15:04:05.000000"),
		self.XRequest.Count, self.XRequest.End.Format("2006-01-02 15:04:05.000000"),
		self.XResponse.Count, self.XResponse.End.Format("2006-01-02 15:04:05.000000"),
	)
}

// GetConn is called before a connection is created or
// retrieved from an idle pool. The hostPort is the
// "host:port" of the target or proxy. GetConn is called even
// if there's already an idle cached connection available.
func (self *Status_t) GetConn(hostPort string) {
	if self.XGetConn.Count++; self.XGetConn.Count == 1 {
		self.XGetConn.Begin = time.Now()
	}
	self.Hosts = append(self.Hosts, hostPort)
}

// GotConn is called after a successful connection is
// obtained. There is no hook for failure to obtain a
// connection; instead, use the error from
// Transport.RoundTrip.
func (self *Status_t) GotConn(in httptrace.GotConnInfo) {
	self.XGotConn.Count++
	self.XGotConn.End = time.Now()
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
		self.Err = err
	}
}

// GotFirstResponseByte is called when the first byte of the response
// headers is available.
func (self *Status_t) GotFirstResponseByte() {
	self.XResponse.Count++
	self.XResponse.End = time.Now()
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
	if self.XDnsStart.Count++; self.XDnsStart.Count == 1 {
		self.XDnsStart.Begin = time.Now()
	}
}

// DNSDone is called when a DNS lookup ends.
func (self *Status_t) DNSDone(in httptrace.DNSDoneInfo) {
	self.XDnsDone.Count++
	self.XDnsDone.End = time.Now()
	if in.Err != nil {
		self.Err = in.Err
	}
}

// ConnectStart is called when a new connection's Dial begins.
// If net.Dialer.DualStack (IPv6 "Happy Eyeballs") support is
// enabled, this may be called multiple times.
func (self *Status_t) ConnectStart(network, addr string) {
	if self.XConnectStart.Count++; self.XConnectStart.Count == 1 {
		self.XConnectStart.Begin = time.Now()
	}
}

// ConnectDone is called when a new connection's Dial
// completes. The provided err indicates whether the
// connection completed successfully.
// If net.Dialer.DualStack ("Happy Eyeballs") support is
// enabled, this may be called multiple times.
func (self *Status_t) ConnectDone(network, addr string, err error) {
	self.XConnectDone.Count++
	self.XConnectDone.End = time.Now()
	if err != nil {
		self.Err = err
	}
}

// TLSHandshakeStart is called when the TLS handshake is started. When
// connecting to an HTTPS site via an HTTP proxy, the handshake happens
// after the CONNECT request is processed by the proxy.
func (self *Status_t) TLSHandshakeStart() {
	if self.XTlsStart.Count++; self.XTlsStart.Count == 1 {
		self.XTlsStart.Begin = time.Now()
	}
}

// TLSHandshakeDone is called after the TLS handshake with either the
// successful handshake's connection state, or a non-nil error on handshake
// failure.
func (self *Status_t) TLSHandshakeDone(in tls.ConnectionState, err error) {
	self.XTlsDone.Count++
	self.XTlsDone.End = time.Now()
	if err != nil {
		self.Err = err
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
	self.XRequest.Count++
	self.XRequest.End = time.Now()
	if in.Err != nil {
		self.Err = in.Err
	}
}
