package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
)

var whitelistedHost = map[string]struct{}{
	"s4.anilist.co":  {},
	"media.kitsu.io": {},
}

func main() {
	lambda.Start(Handler)
}

func Handler(ctx context.Context, request events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	eventUrl := request.QueryStringParameters["url"]
	parsedUrl, err := url.Parse(eventUrl)
	if err != nil {
		return events.APIGatewayProxyResponse{}, err
	}
	if _, ok := whitelistedHost[parsedUrl.Host]; !ok {
		return events.APIGatewayProxyResponse{}, fmt.Errorf("host is not whitelisted: %s", parsedUrl.Host)
	}

	var httpFunc func(url string) (resp *http.Response, err error)
	switch request.HTTPMethod {
	case http.MethodHead:
		httpFunc = http.Head
	default:
		httpFunc = http.Get
	}

	resp, err := httpFunc(eventUrl)
	if err != nil {
		return events.APIGatewayProxyResponse{}, err
	}
	defer resp.Body.Close()

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
			"Content-Type":                 contentType,
			"Access-Control-Allow-Origin":  "*",
			"Access-Control-Allow-Headers": "Content-Type,X-Amz-Date,Authorization,X-Api-Key,X-Amz-Security-Token",
			"Access-Control-Allow-Methods": "GET, HEAD, OPTIONS, POST",
		},
		IsBase64Encoded: true,
		Body:            imgBuffer.String(),
	}, nil
}
