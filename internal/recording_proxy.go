package internal

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"recording-proxy/internal/mongodb"
	"time"
)

type Data struct {
	Headers http.Header
	Body    []byte
}
type RequestResponse struct {
	Url        url.URL
	Request    Data
	Response   Data
	StatusCode int
	Start      time.Time
	End        time.Time
}

type Handler interface {
	Handle(rr *RequestResponse)
}

type RecordingProxy struct {
	customTransport http.RoundTripper
	port            int
	targetSchema    string
	targetHost      string
	handlers        []Handler
}

func NewRecordingProxy(
	port int,
	targetSchema string,
	targetHost string,
) *RecordingProxy {
	// Here, you can customize the transport, e.g., set timeouts or enable/disable keep-alive
	log.Println("Creating proxy server on ", port)
	log.Println("Target schema: ", targetSchema)
	log.Println("Target host: ", targetHost)

	return &RecordingProxy{
		customTransport: http.DefaultTransport,
		port:            port,
		targetSchema:    targetSchema,
		targetHost:      targetHost,
	}
}

func (x *RecordingProxy) AddHandler(storer *mongodb.Storer) {
	x.handlers = append(x.handlers, storer)
}

func (x *RecordingProxy) Run() {
	// Create a new HTTP server with the handleRequest function as the handler
	server := http.Server{
		Addr:    fmt.Sprintf(":%d", x.port),
		Handler: http.HandlerFunc(x.handleRequest),
	}

	// Start the server and log any errors
	log.Println("Starting proxy server on ", x.port)
	err := server.ListenAndServe()
	if err != nil {
		log.Fatal("Error starting proxy server: ", err)
	}

}

func (x *RecordingProxy) handleRequest(w http.ResponseWriter, r *http.Request) {
	// Create a new HTTP request with the same method, URL, and body as the original request
	log.Println("Handling request: ", r.URL)

	rr := RequestResponse{
		Url:     *r.URL,
		Start:   time.Now(),
		Request: Data{Headers: r.Header},
	}

	targetURL := *r.URL
	targetURL.Scheme = x.targetSchema
	targetURL.Host = x.targetHost

	var requestBody bytes.Buffer
	teeRequestReader := io.TeeReader(r.Body, &requestBody)

	proxyReq, err := http.NewRequest(r.Method, targetURL.String(), teeRequestReader)
	if err != nil {
		http.Error(w, "Error creating proxy request", http.StatusInternalServerError)
		return
	}

	// Copy the headers from the original request to the proxy request
	for name, values := range r.Header {
		for _, value := range values {
			proxyReq.Header.Add(name, value)
		}
	}

	// Send the proxy request using the custom transport
	resp, err := x.customTransport.RoundTrip(proxyReq)
	if err != nil {
		http.Error(w, "Error sending proxy request", http.StatusInternalServerError)
		return
	}
	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {
			log.Println("Error closing body: ", err)
		}
	}(resp.Body)

	// Copy the headers from the proxy response to the original response
	for name, values := range resp.Header {
		for _, value := range values {
			w.Header().Add(name, value)
		}
	}

	// Set the status code of the original response to the status code of the proxy response
	w.WriteHeader(resp.StatusCode)

	var responseBody bytes.Buffer

	teeReader := io.TeeReader(resp.Body, &responseBody)

	// Copy the body of the proxy response to the original response and made a copy to a buffer
	_, err = io.Copy(w, teeReader)
	if err != nil {
		log.Println("Error copying proxy response to original response: ", err)
	}

	rr.End = time.Now()
	rr.Request.Body = requestBody.Bytes()
	rr.Response = Data{
		Headers: resp.Header,
		Body:    responseBody.Bytes(),
	}
	rr.StatusCode = resp.StatusCode
	for _, handler := range x.handlers {
		handler.Handle(&rr)
	}
}
