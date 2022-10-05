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
	"github.com/aws/aws-xray-sdk-go/xray"
	"golang.org/x/net/context/ctxhttp"
)

func main() {
	lambda.Start(Handler)
}

func Handler(ctx context.Context, request events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	_, subsegment := xray.BeginSubsegment(ctx, "handling request")
	defer subsegment.Close(nil)

	eventUrl := request.QueryStringParameters["url"]
	parsedUrl, err := url.Parse(eventUrl)
	if err != nil {
		return events.APIGatewayProxyResponse{}, fmt.Errorf("failed to parse url: %w", err)
	}

	if !isWhiteListedHost(parsedUrl.Host) {
		return events.APIGatewayProxyResponse{}, fmt.Errorf("host is not whitelisted: %s", parsedUrl.Host)
	}

	var httpFunc func(ctx context.Context, client *http.Client, url string) (resp *http.Response, err error)
	switch request.HTTPMethod {
	case http.MethodHead:
		httpFunc = ctxhttp.Head
	default:
		httpFunc = ctxhttp.Get
	}

	resp, err := httpFunc(ctx, xray.Client(nil), eventUrl)
	if err != nil {
		return events.APIGatewayProxyResponse{}, fmt.Errorf("failed to make http request: %w", err)
	}
	defer resp.Body.Close()

	_, subsegment2 := xray.BeginSubsegment(ctx, "image decode")
	defer subsegment2.Close(nil)

	contentType := resp.Header.Get("Content-Type")
	if !strings.HasPrefix(contentType, "image/") {
		return events.APIGatewayProxyResponse{}, fmt.Errorf("invalid content type: %s", contentType)
	}

	imgBuffer := new(bytes.Buffer)
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
	for _, host := range whitelistedHosts {
		if strings.EqualFold(host, host) {
			return true
		}
	}
	return false
}
