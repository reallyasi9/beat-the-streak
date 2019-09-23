from google.cloud import firestore, error_reporting
from absl import logging, flags, app
from urllib.request import urlopen
import re
import ssl
import datetime

FS = firestore.Client()
ER = error_reporting.Client()

FLAGS = flags.FLAGS

SAGARIN_URL = "https://sagarin.com/sports/cfsend.htm"

RE_HOME_ADV = re.compile(r"HOME ADVANTAGE=.*?\[<font color=\"#0000ff\">\s*([\-0-9.]+)")
RE_RATINGS = re.compile(r"<font color=\"#000000\">\s+\d+\s+(.*?)\s+[A]+\s*=<.*?<font color=\"#9900ff\">\s*([\-0-9.]+)")


TEAMS_REF = FS.collection("teams")
# collate teams for in-memory searching
TEAMS = {}
for team in TEAMS_REF.stream():
    TEAMS[team.id] = team.to_dict()


def search_long(long):
    docs = []
    for team_id, team in TEAMS.items():
        if "other_names" in team and long in team["other_names"]:
            docs.append(team_id)
    return docs


flags.DEFINE_string("ratings", SAGARIN_URL, "URL of Sagarin Ratings output to parse.")


def parse_sagarin(url):

    # why, Sagarin?
    myssl = ssl.create_default_context()
    myssl.check_hostname=False
    myssl.verify_mode=ssl.CERT_NONE

    home_adv = 0
    ratings = {}

    # Not when the firestore was updated, but when the prediction was downloaded
    timestamp = datetime.datetime.now(datetime.timezone.utc)

    logging.info("download ratings from %s", url)

    with urlopen(url, context=myssl) as u:
        text = u.read().decode("utf-8")
        
        home_adv_match = RE_HOME_ADV.search(text)
        if not home_adv_match:
            logging.fatal("home advantage cannot be parsed")
        
        home_adv = home_adv_match.group(1)
        home_adv = float(home_adv)
        logging.info("Home advantage: %f", home_adv)

        ratings_matches = RE_RATINGS.finditer(text)
        if not ratings_matches:
            logging.fatal("ratings cannot be parsed")
        
        for match in ratings_matches:

            team, rating = match.group(1, 2)
            rating = float(rating)
            logging.info("%s: %f", team, rating)
            ratings[team] = rating
    
    n_errors = 0
    write_me = []
    for team, rating in ratings.items():
        found = search_long(team)
        
        if len(found) == 0:
            err = f"unable to find team `{team}`: update teams.other_names"
            logging.error(err)
            ER.report(err)
            n_errors += 1
            continue
        
        if len(found) > 1:
            err = f"team name `{team}` ambiguous: found {docs}"
            logging.error(err)
            ER.report(err)
            n_errors += 1
            continue
        
        write_me.append({
            "team": team,
            "team_id": found[0],
            "rating": rating,
            })

    if n_errors > 0:
        logging.fatal("correct %d team errors and try again", n_errors)
    
    sags_ref = FS.collection("sagarin")
    doc_ref = sags_ref.document()
    doc_ref.set({
        "timestamp": timestamp,
        "home_advantage": home_adv,
        })
    
    ratings_ref = doc_ref.collection("ratings")
    for rating in write_me:
        ratings_doc = ratings_ref.document()
        ratings_doc.set(rating)



def http(request):
    """HTTP Cloud Function.
    Args:
        request (flask.Request): The request object.
        <http://flask.pocoo.org/docs/1.0/api/#flask.Request>
    Returns:
        The response text, or any set of values that can be turned into a
        Response object using `make_response`
        <http://flask.pocoo.org/docs/1.0/api/#flask.Flask.make_response>.
    """
    request_json = request.get_json(silent=True)
    request_args = request.args

    if request_json and 'ratings' in request_json:
        ratings = request_json['ratings']
    elif request_args and 'ratings' in request_args:
        ratings = request_args['ratings']
    else:
        ratings = SAGARIN_URL
    parse_sagarin(ratings)


def pubsub(event, context):
    """Background Cloud Function to be triggered by Pub/Sub.
    Args:
         event (dict):  The dictionary with data specific to this type of
         event. The `data` field contains the PubsubMessage message. The
         `attributes` field will contain custom attributes if there are any.
         context (google.cloud.functions.Context): The Cloud Functions event
         metadata. The `event_id` field contains the Pub/Sub message ID. The
         `timestamp` field contains the publish time.
    """
    import base64

    logging.info(f"triggered with pubsub event {event}")

    if 'data' in event and event['data']:
        ratings = base64.b64decode(event['data']).decode('utf-8')
    elif 'attributes' in event and event['attributes'] and 'ratings' in event['attributes']:
        ratings = event['attributes']['ratings']
    else:
        ratings = SAGARIN_URL
    parse_sagarin(ratings)


def command_line(argv):
    parse_sagarin(FLAGS.ratings)


if __name__ == '__main__':
    app.run(command_line)