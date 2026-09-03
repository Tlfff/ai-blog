package middleware

import (
	"bytes"
	nethttp "net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

type bodyLogWriter struct {
	gin.ResponseWriter
	body *bytes.Buffer
}

func (w bodyLogWriter) Write(b []byte) (int, error) {
	w.body.Write(b)
	return w.ResponseWriter.Write(b)
}

type Request struct {
	Method string            `json:"method"`
	Path   string            `json:"path"`
	URI    string            `json:"uri"`
	Header map[string]string `json:"header"`
	Body   string            `json:"body"`
}

type Response struct {
	Status int               `json:"status"`
	Header map[string]string `json:"header"`
	Body   string            `json:"body"`
}

type Access struct {
	Time       time.Time `json:"time"`
	ServerID   string    `json:"server_id"`
	ServerPort string    `json:"server_port"`
	RequestID  string    `json:"request_id"`
	Request    Request   `json:"request"`
	Response   Response  `json:"response"`

	Latency   string `json:"latency"`
	LatencyNs int64  `json:"latency_ns"`
}

func toMapString(bean nethttp.Header) (ret map[string]string) {
	ret = make(map[string]string)
	for key, value := range bean {
		ret[key] = strings.Join(value, "\u0020")
	}
	return ret
}
