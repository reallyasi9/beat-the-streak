#!/bin/sh
gcloud --project <PROJECT_NAME> functions deploy tpt_scraper --runtime python37 --trigger-topic <PUBSUB_NAME>
