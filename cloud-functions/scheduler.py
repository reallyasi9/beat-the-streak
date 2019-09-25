import yaml
from google.cloud import firestore, error_reporting
from absl import logging, flags, app
import datetime
import sys

FS = firestore.Client()
ER = error_reporting.Client()
TEAMS_GROUP = FS.collection_group("teams")
def lookup_team(name):
    teams = TEAMS_GROUP.where("other_names", "array_contains", name)
    found = [team.reference for team in teams.stream()]
    
    if len(found) == 0:
        err = f"team {name} not found: update other_names in teams"
        logging.error(err)
        ER.report(err)
        return None
    
    if len(found) > 1:
        err = f"team {name} ambiguous: found {list(f.id for f in found)}"
        logging.error(err)
        ER.report(err)
        return None
    
    return found[0]


def split_locale_team(team):
    if team[0] in "<>@!":
        return team[1:], {"@": -2, ">": -1, "!": 0, "<": 1}[team[0]]
    return team, 2


BYE_WEEK = next(FS.collection("teams").where("name_4", "==", "BYE").limit(1).stream()).reference


FLAGS = flags.FLAGS
flags.DEFINE_string("schedule", None, "Schedule YAML file to parse.")
flags.mark_flag_as_required("schedule")


def schedule(schedule_file):
    with open(schedule_file, "r") as f:
        schedule_yaml = yaml.load(f)
    
    logging.info(f"parsing schedule {schedule_yaml}")

    season_doc = next(FS.collection("seasons").order_by("start", direction="DESCENDING").limit(1).stream())
    season_ref = season_doc.reference

    schedules_ref = FS.collection("schedules")
    schedule_ref = schedules_ref.document()
    schedule_ref.set({"season": season_ref, "timestamp": datetime.datetime.now(datetime.timezone.utc)})

    team_schedules_ref = schedule_ref.collection("teams")

    team_errors = 0
    for team, others in schedule_yaml.items():
        team1 = lookup_team(team)
        if not team1:
            team_errors += 1
            continue

        opponents = []
        locales = []
        for lteam in others:

            if not lteam:
                opponents.append(BYE_WEEK)
                locales.append(0)
                continue

            strip_team, locale = split_locale_team(lteam)
            team2 = lookup_team(strip_team)
            if not team2:
                team_errors += 1
                continue

            opponents.append(team2)
            locales.append(locale)
        
        debug_list = []
        for t, l in zip(opponents, locales):
            if t:
                debug_list.append(f"{t.id}({l})")
            else:
                debug_list.append("BYE")
        logging.info(f"parsed team {team1.id}: {debug_list}")

        team_schedule_ref = team_schedules_ref.document()
        team_schedule_ref.set({
            "team": team1,
            "opponents": opponents,
            "locales": locales
        })

        



def command_line(argv):
    schedule(FLAGS.schedule)


if __name__ == '__main__':
    logging.info(f"triggered with command line arguments {sys.argv}")
    app.run(command_line)