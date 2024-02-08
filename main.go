package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/gqgs/pool"
)

var bufferPool = pool.New[bytes.Buffer]()

func main() {
	lambda.Start(Handler)
}

func Handler(ctx context.Context, request events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	eventUrl, err := base64.RawURLEncoding.DecodeString(request.QueryStringParameters["url"])
	if err != nil {
		return events.APIGatewayProxyResponse{}, fmt.Errorf("failed decoding base64 string: %w", err)
	}

	parsedUrl, err := url.Parse(string(eventUrl))
	if err != nil {
		return events.APIGatewayProxyResponse{}, fmt.Errorf("failed to parse url: %w", err)
	}

	if !isWhiteListedHost(parsedUrl.Host) {
		return events.APIGatewayProxyResponse{}, fmt.Errorf("host is not whitelisted: %s", parsedUrl.Host)
	}

	var httpFunc func(url string) (resp *http.Response, err error)
	switch request.HTTPMethod {
	case http.MethodHead:
		httpFunc = http.Head
	default:
		httpFunc = http.Get
	}

	resp, err := httpFunc(string(eventUrl))
	if err != nil {
		return events.APIGatewayProxyResponse{}, fmt.Errorf("failed to make http request: %w", err)
	}
	defer resp.Body.Close()

	contentType := resp.Header.Get("Content-Type")
	if !strings.HasPrefix(contentType, "image/") && !strings.HasPrefix(contentType, "application/json") {
		return events.APIGatewayProxyResponse{}, fmt.Errorf("invalid content type: %s", contentType)
	}

	imgBuffer := bufferPool.Get()
	defer bufferPool.Put(imgBuffer)
	imgBuffer.Reset()

	encoder := base64.NewEncoder(base64.StdEncoding, imgBuffer)
	if _, err := io.Copy(encoder, resp.Body); err != nil {
		return events.APIGatewayProxyResponse{}, fmt.Errorf("error reading from resp.Body: %w", err)
	}

	if err := encoder.Close(); err != nil {
		return events.APIGatewayProxyResponse{}, fmt.Errorf("error closing base64 encoder: %w", err)
	}

	return events.APIGatewayProxyResponse{
		StatusCode: http.StatusOK,
		Headers: map[string]string{
			"Content-Type": contentType,
		},
		IsBase64Encoded: true,
		Body:            imgBuffer.String(),
	}, nil
}

func isWhiteListedHost(host string) bool {
	whitelistedHosts := strings.Split(os.Getenv("WHITELISTED_HOSTS"), ",")
	for _, whitelistedHost := range whitelistedHosts {
		if strings.EqualFold(whitelistedHost, host) {
			return true
		}
	}
	return false
}
