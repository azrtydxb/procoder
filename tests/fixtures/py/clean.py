# Clean fixture — real near-misses for every rule py.js implements, all of
# which must stay silent.

import hashlib
import subprocess
import yaml
from pprint import pprint

import logging

logger = logging.getLogger(__name__)


def lookup_user(cursor, uid):
    return cursor.execute("SELECT * FROM t WHERE id = %s", (uid,))


def run_command(target):
    subprocess.run(["ls", target])


def load_config(text):
    return yaml.safe_load(text)


def hash_password(password):
    return hashlib.sha256(password.encode())


def insecure_request(session):
    session.get("https://example.com", verify=True)


def add(item, into=None):
    if into is None:
        into = []
    into.append(item)
    return into


def careful():
    try:
        go()
    except ValueError as e:
        logger.exception(e)
        raise


def dump(data):
    pprint(data)


def startup():
    logger.info("started")


def shallow(a, b):
    return a + b


# Documentation that warns against a practice must not be flagged for the
# practice: every rule py.js has, named in prose, still silent.
#
#   never use eval(user_input) or exec(payload) here
#   never pass shell=True, and never call os.system("rm " + target)
#   never cursor.execute(f"SELECT * FROM t WHERE id = {uid}")
#   never pickle.loads(payload) or yaml.load(text)
#   never hashlib.md5(secret) and never verify=False
#   no leftover print("here") or breakpoint()
#   `except:` bare, or `except Exception:` over `pass`, is never acceptable
def documented(value):
    """Do not call eval(value) here, and never build SQL as
    cursor.execute(f"SELECT {value}") — parameterize instead.
    """
    return value


def lookup_user_bound(cursor, uid):
    q = "SELECT * FROM t WHERE id = %s"
    return cursor.execute(q, (uid,))


def list_columns(cursor):
    q = "SELECT " + "id, name" + " FROM t"
    return cursor.execute(q)


def rebuild_query(cursor, uid):
    q = "SELECT * FROM t WHERE id = " + uid
    q = "SELECT * FROM t"
    return cursor.execute(q)


def describe_dir(target):
    cmd = "ls " + target
    logger.info(cmd)
