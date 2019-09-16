import urllib.request
import csv
import io
import datetime
import pandas as pd
import numpy as np
import bs4
from types import SimpleNamespace

from google.cloud import firestore

db = firestore.Client()

WEEKLY_PREDICTIONS_URL = "https://www.thepredictiontracker.com/ncaapredictions.csv"
RESULTS_URL = "https://www.thepredictiontracker.com/ncaaresults.php"


def prediction_iterator():
    with urllib.request.urlopen(WEEKLY_PREDICTIONS_URL) as csvfile:
        wrapper = io.TextIOWrapper(csvfile, encoding="utf-8")
        reader = csv.DictReader(wrapper)
        for row in reader:
            for key, val in row.items():
                if val == "":
                    row[key] = None
                elif key not in ["home", "road"]:
                    row[key] = float(val)
            yield dict(row)


def results_iterator():
    with urllib.request.urlopen(RESULTS_URL) as resultsfile:
        bs = bs4.BeautifulSoup(resultsfile, "lxml")
        results_df = pd.read_html(str(bs), attrs={"class": "results_table"}, header=0)
        results_df = results_df[0]
        results_df.rename(columns={
            "Bias": "bias",
            "Rank": "rank",
            "System": "system",
            "Pct. Correct": "pct_correct",
            "Against Spread": "pct_against_spread",
            "Absolute Error": "mae",
            "Mean Square Error": "mse",
        }, inplace=True)
        results_df["std_dev"] = np.sqrt(results_df["mse"] - results_df["bias"].pow(2))
        for row in results_df.itertuples(index=False):
            yield row._asdict()


def tpt_scraper(event, context):

    # print(f"TPT Scrape triggered by messageId {context.event_id} published at {context.timestamp}")
    if event["attributes"] and "collectionName" in event["attributes"]:
        collection_name = event["attributes"]["collectionName"]
    else:
        collection_name = "thepredictiontracker"

    pred_ref = db.collection(collection_name)

    # Not when the firestore was updated, but when the prediction was downloaded
    timestamp = datetime.datetime.now(datetime.timezone.utc)

    # Each prediction set is a document, with a collection of predictions underneath
    doc_ref = pred_ref.document()
    doc_ref.set({
        "timestamp": timestamp,
        "event_id": context.event_id,
        "event_timestamp": context.timestamp,
        })

    subpred_ref = doc_ref.collection("predictions")

    for row in prediction_iterator():
        subdoc_ref = subpred_ref.document()
        subdoc_ref.set(row)

    subperf_ref = doc_ref.collection("modelperformance")

    for row in results_iterator():
        subdoc_ref = subperf_ref.document()
        subdoc_ref.set(row)



if __name__ == "__main__":
    collection_name = "test_thepredictiontracker"
    event = {
        "attributes": {
            "collectionName": collection_name
        }
    }
    context = SimpleNamespace(event_id="",
        timestamp=datetime.datetime.now(datetime.timezone.utc).isoformat(),
        event_type="test",
        resource="")
    tpt_scraper(event, context)

    # test
    last_week = datetime.datetime.now() - datetime.timedelta(days=7)
    pred_ref = db.collection(collection_name)
    query_ref = (
        pred_ref.where("timestamp", ">", last_week)
        .order_by("timestamp", "DESCENDING")
        .limit(1)
    )
    results = query_ref.stream()
    for doc in results:
        print(f"{doc.id}: {doc.to_dict()}")
        collection_stream = db.collection(
            collection_name, doc.id, "predictions"
        ).stream()
        for d in collection_stream:
            print(f" -- {d.id}: {d.to_dict()}")
        collection_stream = db.collection(
            collection_name, doc.id, "modelperformance"
        ).stream()
        for d in collection_stream:
            print(f" -- {d.id}: {d.to_dict()}")
