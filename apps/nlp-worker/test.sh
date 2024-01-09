#!/bin/bash

# https://cloud.google.com/eventarc/docs/workflows/cloudevents#cloud-storage

#!/bin/bash

curl localhost:8080/Handler \
  -X POST \
  -H "Content-Type: application/json" \
  -H "ce-id: 123451234512345" \
  -H "ce-specversion: 1.0" \
  -H "ce-time: 2020-01-02T12:34:56.789Z" \
  -H "ce-type: google.cloud.storage.object.v1.finalized" \
  -H "ce-source:  //storage.googleapis.com/projects/_/buckets/foo-bar" \
  -d '{
    "bucket": "sample-bucket",
    "contentType": "text/plain",
    "crc32c": "rTVTeQ==",
    "etag": "CNHZkbuF/ugCEAE=",
    "generation": "1587627537231057",
    "id": "sample-bucket/folder/Test.cs/1587627537231057",
    "kind": "storage#object",
    "md5Hash": "kF8MuJ5+CTJxvyhHS1xzRg==",
    "mediaLink": "https://www.googleapis.com/download/storage/v1/b/sample-bucket/o/folder%2FTest.cs?generation=1587627537231057\u0026alt=media",
    "metageneration": "1",
    "name": "folder/Test.cs",
    "selfLink": "https://www.googleapis.com/storage/v1/b/sample-bucket/o/folder/Test.cs",
    "size": "352",
    "storageClass": "MULTI_REGIONAL",
    "timeCreated": "2020-04-23T07:38:57.230Z",
    "timeStorageClassUpdated": "2020-04-23T07:38:57.230Z",
    "updated": "2020-04-23T07:38:57.230Z"
  }'
    
  
            
