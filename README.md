# go-gcp-doc-ai

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

## Back-of-the-envelope calculations

### Quotas, Limitations and Configurations

- https://cloud.google.com/document-ai/quotas

  - Files per batch processing request: 5,000
  - Maximum pages (batch/offline/asynchronous requests): 500
  - Concurrent batch process requests per processor (EU): 5 per project

- https://cloud.google.com/functions/docs/configuring/max-instances

  - Default concurrent instance limit for 2nd gen Cloud Functions (both HTTP and event-driven functions): 100

- Average async document ai OCR duration for batch of 50 (this considering the nature of the documents the author is dealing with. yours might differ): 30s

### Assumptions and Calculations

- Concurrent worker functions/document ai batch processes (1:1 mapping): 5

#### batch 100

- Files per batch processing request: 100
- OCR per 30s: 5\*100=500
- Expected completed OCR per 30s, 1min, 1hour, 1day: `5*100=500`, `1000`, `60k`, `1.44M`
- Expected duration to process 2.5M pages: 42h

### Natural Language

- https://cloud.google.com/natural-language/quotas
