compile:
	CGO_ENABLED=0 goos=linux GOARCH=arm64 go build -tags lambda.norpc -o imgproxy && 7z a imgproxy.zip imgproxy
