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


PICKERS_GROUP = FS.collection_group("pickers")
def lookup_picker(name):
    pickers = PICKERS_GROUP.where("name_luke", "==", name)
    found = [picker.reference for picker in pickers.stream()]

    if len(found) == 0:
        err = f"picker {name} not found: update name_luke in pickers"
        logging.error(err)
        ER.report(err)
        return None
    
    if len(found) > 1:
        err = f"picker {name} ambiguous: found {list(f.id for f in found)}"
        logging.error(err)
        ER.report(err)
        return None
    
    return found[0]


FLAGS = flags.FLAGS
flags.DEFINE_string("remaining", None, "Picker team remaining YAML file to parse.")
flags.DEFINE_string("types", None, "Picker picks remaining YAML file to parse.")
flags.DEFINE_integer("week", None, "Week of picks (starting at 0 for the preseason)")
flags.mark_flag_as_required("remaining")
# flags.mark_flag_as_required("types")
flags.mark_flag_as_required("week")


def streakers(remaining_file, types_file, week):
    with open(remaining_file, "r") as f:
        remaining_yaml = yaml.load(f)
    
    types_yaml = None  # figure it out later: assume straight picks all the way through
    if types_file:
        with open(types_file, "r") as f:
            types_yaml = yaml.load(f)

    
    logging.info(f"parsing remaining {remaining_yaml}")
    if types_file:
        logging.info(f"parsing pick types remaining {types_file}")

    season_doc = next(FS.collection("seasons").order_by("start", direction="DESCENDING").limit(1).stream())
    season_ref = season_doc.reference
    logging.info(f"most recent season: {season_ref.id}")

    picks_ref = FS.collection("picks")
    pick_ref = picks_ref.document()
    pick_ref.set({"season": season_ref, "week": week, "timestamp": datetime.datetime.now(datetime.timezone.utc)})

    streaks_ref = pick_ref.collection("streaks")

    picker_errors = 0
    team_errors = 0
    types_errors = 0

    write_me = []
    for picker, teams in remaining_yaml.items():
        picker_ref = lookup_picker(picker)
        if not picker_ref:
            picker_errors += 1
            continue

        remaining = []
        for team in teams:

            team_ref = lookup_team(team)
            if not team_ref:
                team_errors += 1
                continue

            remaining.append(team_ref)
        
        logging.info(f"parsed picks remaining {picker_ref.id}: {list(team.id for team in remaining)}")

        types = []
        if not types_yaml or picker not in types_yaml:
            types = [0, len(remaining)]
            logging.info(f"assuming single picks for remaining {len(remaining)} teams")
        else:
            types = types_yaml[picker]
        
        sum_types = 0
        for i, n in enumerate(types):
            sum_types += i*n
        if sum_types != len(remaining):
            err = f"pick types remaining for picker {picker_ref.id} {types} does not sum to number of teams remaining {len(remaining)}"
            ER.report(err)
            logging.error(err)
            types_errors += 1
            continue

        write_me.append({
            "picker": picker_ref,
            "remaining": remaining,
            "pick_types_remaining": types
        })

    if picker_errors + team_errors + types_errors > 0:
        logging.fatal(f"fix {picker_errors} picker errors, {team_errors} team errors, and {types_errors} remaining types errors")
        return

    for streak in write_me:
        logging.info(f"setting picks for {streak['picker'].id}")
        streak_ref = streaks_ref.document()
        streak_ref.set(streak)



def command_line(argv):
    streakers(FLAGS.remaining, FLAGS.types, FLAGS.week)


if __name__ == '__main__':
    logging.info(f"triggered with command line arguments {sys.argv}")
    app.run(command_line)