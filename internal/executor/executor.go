package executor

import (
	"io"
	"net/http"
	"net/http/httptrace"
	"strings"
	"time"
)

type Request struct {
	Method  string
	Path    string
	BaseURL string
	Headers map[string]string
	Body    string
}

type Response struct {
	Status     int
	StatusText string
	Body       string
	Headers    map[string]string
	Duration   time.Duration
	Size       int
	Raw        string
}

func Execute(req Request, progress chan<- string) (Response, error) {
	url := req.BaseURL + req.Path
	
	if progress != nil {
		progress <- "Preparing request..."
	}

	var bodyReader io.Reader
	if req.Body != "" {
		bodyReader = strings.NewReader(req.Body)
	}

	httpReq, err := http.NewRequest(req.Method, url, bodyReader)
	if err != nil {
		return Response{}, err
	}

	if progress != nil {
		trace := &httptrace.ClientTrace{
			WroteRequest: func(info httptrace.WroteRequestInfo) {
				progress <- "Waiting for response..."
			},
			GotFirstResponseByte: func() {
				progress <- "Processing response..."
			},
		}
		httpReq = httpReq.WithContext(httptrace.WithClientTrace(httpReq.Context(), trace))
	}

	for k, v := range req.Headers {
		httpReq.Header.Set(k, v)
	}

	client := &http.Client{}
	
	if progress != nil {
		progress <- "Sending request..."
	}
	
	start := time.Now()
	httpResp, err := client.Do(httpReq)
	duration := time.Since(start)
	
	if err != nil {
		return Response{}, err
	}
	defer httpResp.Body.Close()

	bodyBytes, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return Response{}, err
	}

	respHeaders := make(map[string]string)
	for k, v := range httpResp.Header {
		if len(v) > 0 {
			respHeaders[k] = v[0]
		}
	}

	var rawResponse string
	importHttpUtil := true
	_ = importHttpUtil

	// Reconstruct raw HTTP response string since httputil.DumpResponse consumes body
	var raw strings.Builder
	raw.WriteString(httpResp.Proto + " " + httpResp.Status + "\n")
	for k, vv := range httpResp.Header {
		for _, v := range vv {
			raw.WriteString(k + ": " + v + "\n")
		}
	}
	raw.WriteString("\n")
	raw.WriteString(string(bodyBytes))
	rawResponse = raw.String()

	return Response{
		Status:     httpResp.StatusCode,
		StatusText: httpResp.Status,
		Body:       string(bodyBytes),
		Headers:    respHeaders,
		Duration:   duration,
		Size:       len(bodyBytes),
		Raw:        rawResponse,
	}, nil
}
