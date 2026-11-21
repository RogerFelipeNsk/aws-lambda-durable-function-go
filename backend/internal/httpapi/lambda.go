package httpapi

import (
	"bytes"
	"context"
	"encoding/base64"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"

	"github.com/aws/aws-lambda-go/events"
)

func LambdaAdapter(h http.Handler) func(context.Context, events.APIGatewayV2HTTPRequest) (events.APIGatewayV2HTTPResponse, error) {
	return func(ctx context.Context, req events.APIGatewayV2HTTPRequest) (events.APIGatewayV2HTTPResponse, error) {
		httpReq, err := incoming(ctx, req)
		if err != nil {
			return events.APIGatewayV2HTTPResponse{StatusCode: http.StatusInternalServerError}, err
		}
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httpReq)
		return outgoing(rec.Result()), nil
	}
}

func incoming(ctx context.Context, req events.APIGatewayV2HTTPRequest) (*http.Request, error) {
	path := req.RawPath
	if path == "" {
		path = "/"
	}
	u := url.URL{Scheme: "https", Path: path, RawQuery: req.RawQueryString}
	var body io.Reader = strings.NewReader(req.Body)
	if req.IsBase64Encoded && req.Body != "" {
		raw, err := base64.StdEncoding.DecodeString(req.Body)
		if err != nil {
			return nil, err
		}
		body = bytes.NewReader(raw)
	}
	httpReq, err := http.NewRequestWithContext(ctx, req.RequestContext.HTTP.Method, u.String(), body)
	if err != nil {
		return nil, err
	}
	for k, v := range req.Headers {
		httpReq.Header.Set(k, v)
	}
	return httpReq, nil
}

func outgoing(res *http.Response) events.APIGatewayV2HTTPResponse {
	defer res.Body.Close()
	raw, _ := io.ReadAll(res.Body)
	headers := map[string]string{}
	for k, vs := range res.Header {
		headers[k] = strings.Join(vs, ",")
	}
	return events.APIGatewayV2HTTPResponse{
		StatusCode: res.StatusCode,
		Headers:    headers,
		Body:       string(raw),
	}
}
