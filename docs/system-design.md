Data Pipeline System Design

# 1. Buckets:

The buckets involved in the data pipeline.

## src Bucket

Contains the original 2 million images for processing.

## src-refs Bucket

Used for deduplication. It stores files named after the CRC32C checksum of each image from the src bucket, with each file's content being the name of the corresponding image file.

## ocr-err Bucket

Where errors encountered during OCR processing by the OCRWorker function are logged in JSON format.

## ocr-output Bucket

Stores the JSON files generated by the OCRWorker function, which are the results of the Document AI OCR processing.

## nlp-err Bucket

Where errors encountered during NLP processing by the NLPWorker function are logged in JSON format.

## nlp-output Bucket

Receives the JSON responses from the NLPWorker function, containing the results of the NLP analysis.

These buckets form the core storage components of your data pipeline, handling source data, deduplication references, processing outputs, and error logging.

# 2. Cloud Functions

## Dispatcher Function

- Trigger: Manually initiated.
- Function: Iterates through images in src bucket, checks src-refs for deduplication, and batches 100 images for processing.
- Output: Sends batches to Pub/Sub topic.
- Concurrency: Limited to 1 instance.

## OCRWorker Function

- Trigger: Pub/Sub events.
- Function: Processes images using Document AI OCR, handles errors, and outputs to ocr-output bucket.
- Error Handling: Writes errors to ocr-err bucket.
- Concurrency: Limited to 5 concurrent instances.

## NLPWorker Function

- Trigger: Eventarc file creation events in ocr-output bucket.
- Function: Reads OCR results, filters text, processes with NLP (AnalyzeEntitiesRequest), handles errors, and outputs to nlp-output bucket.
- Error Handling: Writes errors to nlp-err bucket.

# 3. Pub/Sub and Eventarc

- Pub/Sub Topic: Used for queuing batches of filenames from the Dispatcher to the OCRWorker.
- Eventarc: Triggers the NLPWorker function upon file creation in the ocr-output bucket.

# 4. Error Handling and Retry Logic:

- Robust error handling and retry mechanisms in both OCRWorker and NLPWorker functions.
- Use of exponential backoff and dead-letter queues for efficient retry strategies.

# 5. Performance and Scaling:

- Concurrency control in Cloud Functions to manage workload.
- Monitoring and alerting integrated for performance tracking and issue identification.
