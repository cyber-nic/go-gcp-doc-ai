build:
	go build -o ./bin/app ./src

run-worker:
	go run ./worker-cmd

build-worker:
	go build ./worker-cmd -o ./bin/worker

debug:
	go run ./src -debug=true

test:
	go test -v ./src/...

cover:
	go test -coverprofile=coverage.out && go tool cover -html=coverage.out