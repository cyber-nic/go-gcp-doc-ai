# The Why: Revising the Hashing Strategy

The author initially used Google Cloud Platform's (GCP) provided CRC32 value for identifying duplicate images in a storage bucket, which proved insufficient. This approach faced two main issues:

## CRC32 Limitations

CRC32, particularly the CRC32C variant used by Google Cloud Storage (GCS), is optimized for error detection, not as a unique identifier for data. It has a higher probability of collisions (1 in 4.3 billion) compared to more robust algorithms.
The CRC32 values were inconsistent across distinct GCP objects, even for identical images.

## Choosing SHA-256

To overcome these limitations, SHA-256, a more robust hashing algorithm, was chosen. SHA-256 significantly reduces the likelihood of hash collisions, ensuring better uniqueness for data identification.

# The Deduper: Enhancements and Performance

The Deduper application, initially a Cloud Function, was later reconfigured to run as a standalone process on a virtual machine (VM). This change addressed the time-out limitations (9 minutes) of Cloud Functions.

## Efficiency and Scalability

Running on a cost-effective Linux VM in the same region as the GCP bucket, the application processes images at an average rate of 180ms per image, roughly translating to about 480,000 images per day. This setup meets the author's performance requirements.

## Checkpointing Mechanism

The application implements a checkpointing system. It records the name of every nth processed file (determined by the `PROGRESS_COUNT` environment variable) in a 'checkpoint' file within a designated storage bucket.

## Authentication and Permission Management

Permissions are seamlessly managed using integrated authentication via the gcloud SDK, streamlining access control.

## Firestore Document Creation and Duplication Avoidance

For each processed file, a file document is created in the Firestore collection specified by `FIRESTORE_FILE_COLLECTION_NAME`. This document stores the image's hash and uses the GCP bucket object name as its identifier.
The application checks for the existence of this document before downloading an image, preventing redundant processing.
Content-Based Hashing (CBH) and Image Data Storage:

The SHA-256 hash serves as a key in a separate Firestore collection (`FIRESTORE_IMAGE_COLLECTION_NAME`). Each image document stores vital image metadata (height, width, pixels) and a list of filenames of duplicate images.

## Resulting Firestore Collection

The outcome is a comprehensive Firestore collection (`FIRESTORE_IMAGE_COLLECTION_NAME`) representing unique images. This collection can be utilized by the Dispatcher application, facilitating further data management and processing tasks.

# Summary

In summary, the Deduper application is an optimized, scalable solution for identifying and managing duplicate images in GCP storage, leveraging robust hashing and efficient data handling techniques.

# Configuration

```
# GCP project id
GCP_PROJECT_ID=my-project

# name of bucket in which to store the checkpoint file
BUCKET_CHECKPOINT_NAME=checkpoint-bucket

# name of bucket with images to "dedup"
BUCKET_NAME=source-data-bucket

# prefix that can be used to process a given path
BUCKET_PREFIX="**/*.jpg"

# firestore database name
FIRESTORE_DATABASE_ID="(default)"

# name of the firestore collection to store image documents (height, width, pixels, filenames)
FIRESTORE_IMAGE_COLLECTION_NAME=images

# name of the firestore collection to store file documents (image hash)
FIRESTORE_FILE_COLLECTION_NAME=files

# total number of files to process before process exits
MAX_FILES=14

# number of files before writing a new checkpoint and logging progress
PROGRESS_COUNT=5
```
