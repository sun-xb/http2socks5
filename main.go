// http2socks5 project main.go
package main

import (
	"flag"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"

	"golang.org/x/net/proxy"
)

type HttpProxyHandler struct {
	dialer proxy.Dialer
}

func (h *HttpProxyHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	hijack, ok := w.(http.Hijacker)
	if !ok {
		http.Error(w, "webserver doesn't support hijacking", http.StatusInternalServerError)
		return
	}
	port := r.URL.Port()
	if port == "" {
		port = "80"
	}
	socksConn, err := h.dialer.Dial("tcp", r.URL.Hostname()+":"+port)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	httpConn, _, err := hijack.Hijack()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if r.Method == http.MethodConnect {
		httpConn.Write([]byte("HTTP/1.1 200 Connection Established\r\n\r\n"))
	} else {
		r.Write(socksConn)
	}

	pipeConn := func(w, r net.Conn) {
		io.Copy(w, r)
		if c, ok := w.(*net.TCPConn); ok {
			c.CloseWrite()
		}
		if c, ok := r.(*net.TCPConn); ok {
			c.CloseRead()
		}
	}
	go pipeConn(socksConn, httpConn)
	go pipeConn(httpConn, socksConn)
}

func main() {
	httpAddr := flag.String("http", "127.0.0.1:8118", "local http proxy address")
	socks5Addr := flag.String("socks5", "socks5://127.0.0.1:1080", "remote socks5 address")
	flag.Parse()

	socksUrl, err := url.Parse(*socks5Addr)
	if err != nil {
		log.Fatalln("proxy url parse error:", err)
	}
	dialer, err := proxy.FromURL(socksUrl, proxy.Direct)
	if err != nil {
		log.Fatalln("can not make proxy dialer:", err)
	}
	if err := http.ListenAndServe(*httpAddr, &HttpProxyHandler{dialer}); err != nil {
		log.Fatalln("can not start http server:", err)
	}
}
