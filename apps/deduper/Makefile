build:
	go build -o ./bin/app ./

test:
	go test -v ./...

run:
	gow run ./

invoke:
	curl -X POST $DEDUPER_URL -H "Authorization: bearer $(gcloud auth print-identity-token)"