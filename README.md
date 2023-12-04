# go-gcp-docai-ocr

This project is a quick and effective way to OCR files in a GCP bucket. It attempts to handle failures by tracking already processed files enabling itself to pickup where it leftoff. The project makes reasonable performance for cost tradeoffs such as tracking processed files using a bucket rather than a database.

- The application is tailored to operated in GCP.
- The code is deployed as a Cloud Function.
- The application:
  - reads from an `src` bucket
  - reads and writes image CRC32 hashes to a `refs` bucket
  - utilizes the Document AI OCR batch capabilities, which writes to a `dst` bucket
  - writes errors to an `err` bucket
- The application can be triggered/invoked with via http. It runs until completion

https://cloud.google.com/functions/docs/running/functions-emulator#cloudevent-function
