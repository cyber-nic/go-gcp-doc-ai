# Local Test

https://cloud.google.com/functions/docs/running/overview

```
go get github.com/GoogleCloudPlatform/functions-framework-go/funcframework@v1.8.0

# optional, default 8080
export PORT=5000
```

# Cloud Emulator

https://cloud.google.com/functions/docs/running/functions-emulator#http-function

```
gcloud alpha functions local deploy worker \
 --entry-point="workerEntrypoint" \
 --runtime=go121
```
