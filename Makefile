build:
	go build -o ./bin/app ./src

test:
	go test -v ./worker/...

# run-worker
rw:
	go run ./apps/ocr-worker/cmd

run-worker:
	go run ./apps/ocr-worker/cmd

debug:
	go run ./src -debug=true

build-worker:
	go build ./worker-cmd -o ./bin/worker


cover:
  coverage.out && go tool cover -html=coverage.out