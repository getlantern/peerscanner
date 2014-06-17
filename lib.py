import os
import traceback
from functools import wraps

from flask import abort, request
import redis
import pyflare


app = None
AUTH_TOKEN = os.getenv('AUTH_TOKEN')
debug = os.getenv('DEBUG') == 'true'
do_check_auth = False


def login_to_redis():
    return redis.from_url(os.environ['REDISCLOUD_URL'])

def login_to_cloudflare():
    return pyflare.Pyflare(os.environ['CLOUDFLARE_USER'],
                           os.environ['CLOUDFLARE_API_KEY'])

def get_param(name):
    return request.args.get(name, request.form.get(name))

def check_auth():
    if do_check_auth and get_param('auth-token') != AUTH_TOKEN:
        abort(403)

def checks_auth(fn):
    @wraps(fn)
    def deco(*args, **kw):
        check_auth()
        return fn(*args, **kw)
    return deco

def log_tracebacks(fn):
    @wraps(fn)
    def deco(*args, **kw):
        try:
            return fn(*args, **kw)
        except:
            return "<pre>" + traceback.format_exc() + "</pre>"
    return deco

def check_and_route(*args, **kw):
    def deco(fn):
        ret = checks_auth(fn)
        if debug:
            ret = log_tracebacks(ret)
        return app.route(*args, **kw)(ret)
    return deco
