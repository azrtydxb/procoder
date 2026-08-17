# Deliberately unsafe/broken fixture — exercises every py.js finding id.

import hashlib
import os
import pickle
import subprocess
import yaml


def lookup_user(cursor, uid):
    return cursor.execute(f"SELECT * FROM t WHERE id = {uid}")


def run_command(cmd, target):
    os.system("rm " + target)
    subprocess.run(cmd, shell=True)


def run_payload(user_input):
    eval(user_input)


def load_blob(payload):
    return pickle.loads(payload)


def hash_password(password):
    return hashlib.md5(password.encode())


def insecure_request(session):
    session.get("https://example.com", verify=False)


def add(item, into=[]):
    into.append(item)
    return into


def bare():
    try:
        go()
    except:
        pass


def swallowed():
    try:
        go()
    except Exception:
        pass


def debug_print():
    print("here")
    breakpoint()


def deep(x, y, z, w):
    if x:
        for a in y:
            while z:
                go(w)


def load_profile(url):
    s = requests.Session()
    return s.get(url)


def lookup_user_tainted(cursor, uid):
    q = f"SELECT * FROM t WHERE id = {uid}"
    return cursor.execute(q)
