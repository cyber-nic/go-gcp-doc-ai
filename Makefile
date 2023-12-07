build:
	go build -o ./bin/app ./src

test:
	go test -v ./worker/...

# run-worker
rw:
	go run ./worker-cmd

run-worker:
	go run ./worker-cmd

debug:
	go run ./src -debug=true

build-worker:
	go build ./worker-cmd -o ./bin/worker


cover:
            { "Name": "coverage.out && go tool cover -html=coverage.out