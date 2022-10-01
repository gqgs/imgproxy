compile:
	CGO_ENABLED=0 goos=linux GOARCH=amd64 go build -o imgproxy && 7z a imgproxy.zip imgproxy
