compile:
	CGO_ENABLED=0 goos=linux GOARCH=arm64 go build -tags lambda.norpc -ldflags="-s -w" -o bootstrap && 7z a -mm=Deflate -mfb=258 -mpass=15 imgproxy.zip bootstrap
