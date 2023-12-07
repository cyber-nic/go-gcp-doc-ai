#!/bin/bash


curl localhost:8080/worker \
  -X POST \
  -H "Content-Type: application/json" \
  -H "ce-id: 123451234512345" \
  -H "ce-specversion: 1.0" \
  -H "ce-time: 2020-01-02T12:34:56.789Z" \
  -H "ce-type: google.cloud.pubsub.topic.v1.messagePublished" \
  -H "ce-source: //pubsub.googleapis.com/projects/MY-PROJECT/topics/MY-TOPIC" \
  -d '{
        "message": {
          "attributes": {
            "foo": "far",
            "boo": "bar"
          },
          "data": [
            { "Name": "foo/bar_001.jpg" },
            { "Name": "foo/bar_002.jpg" },
            { "Name": "foo/bar_003.jpg" },
            { "Name": "foo/bar_004.jpg" },
            { "Name": "foo/bar_005.jpg" },
            { "Name": "foo/bar_006.jpg" },
            { "Name": "foo/bar_007.jpg" },
            { "Name": "foo/bar_008.jpg" },
            { "Name": "foo/bar_009.jpg" },
            { "Name": "foo/bar_010.jpg" },
            { "Name": "foo/bar_011.jpg" },
            { "Name": "foo/bar_012.jpg" },
            { "Name": "foo/bar_013.jpg" },
            { "Name": "foo/bar_014.jpg" },
            { "Name": "foo/bar_015.jpg" },
            { "Name": "foo/bar_016.jpg" },
            { "Name": "foo/bar_017.jpg" },
            { "Name": "foo/bar_018.jpg" },
            { "Name": "foo/bar_019.jpg" },
            { "Name": "foo/bar_020.jpg" }
          ]
        },
        "subscription": "projects/MY-PROJECT/subscriptions/MY-SUB"
      }'

            
