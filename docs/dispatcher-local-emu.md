# setup

```
export PROJECT_ID=doc-ai
export TOPIC_NAME=ocr-dispatch
```

# instructions

https://cloud.google.com/functions/docs/local-development

## pubsub emu

In a first terminal, start the Pub/Sub emulator on port 8043 in a local project:

```
gcloud beta emulators pubsub start --project=$PROJECT_ID --host-port='localhost:8043'
```

## create topic

In a second terminal, create a Pub/Sub topic and subscription:

```
curl -s -X PUT "http://localhost:8043/v1/projects/${PROJECT_ID}/topics/${TOPIC_NAME}"
```
