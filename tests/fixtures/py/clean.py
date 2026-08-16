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
