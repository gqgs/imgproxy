package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"io"
	"log/slog"
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
		slog.Error("failed decoding base64 string", "error", err.Error())
		return events.APIGatewayProxyResponse{}, errors.New("failed decoding base64 string")
	}
	slog.Info("processing request", "url", eventUrl)

	parsedUrl, err := url.Parse(string(eventUrl))
	if err != nil {
		slog.Error("failed to parse url", "error", err.Error())
		return events.APIGatewayProxyResponse{}, errors.New("failed to parse url")
	}

	if !isWhiteListedHost(parsedUrl.Host) {
		slog.Error("host is not whitelisted", "host", parsedUrl.Host)
		return events.APIGatewayProxyResponse{}, errors.New("host is not whitelisted")
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
		slog.Error("failed to make http request", "error", err.Error())
		return events.APIGatewayProxyResponse{}, errors.New("failed to make http request")
	}
	defer resp.Body.Close()

	contentType := resp.Header.Get("Content-Type")
	if !strings.HasPrefix(contentType, "image/") && !strings.HasPrefix(contentType, "application/json") {
		slog.Error("invalid content type", "mime-type", contentType)
		return events.APIGatewayProxyResponse{}, errors.New("failed to make http request")
	}

	imgBuffer := bufferPool.Get()
	defer bufferPool.Put(imgBuffer)
	imgBuffer.Reset()

	encoder := base64.NewEncoder(base64.StdEncoding, imgBuffer)
	if _, err := io.Copy(encoder, resp.Body); err != nil {
		slog.Error("error reading from resp.Body", "error", err.Error())
		return events.APIGatewayProxyResponse{}, errors.New("error reading from resp.Body")
	}

	if err := encoder.Close(); err != nil {
		slog.Error("error closing base64 encoder", "error", err.Error())
		return events.APIGatewayProxyResponse{}, errors.New("error closing base64 encoder")
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
