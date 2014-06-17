import os
import traceback
from functools import wraps

from flask import abort, Flask, request
from pyflare import Pyflare
import redis as redis_


AUTH_TOKEN = os.getenv('AUTH_TOKEN')
app = Flask(__name__)

debug = os.getenv('DEBUG') == 'true'

if debug:
    methods = ['POST', 'GET']
else:
    methods = ['POST']

def login_to_redis():
    #url = urlparse.urlparse(os.environ['REDISCLOUD_URL'])
    #return Redis(host=url.hostname, port=url.port, password=url.password)
    return redis_.from_url(os.environ['REDISCLOUD_URL'])

def login_to_cloudflare():
    return Pyflare(os.environ['CLOUDFLARE_USER'],
                   os.environ['CLOUDFLARE_API_KEY'])

redis = login_to_redis()
cloudflare = login_to_cloudflare()

def check_auth():
    if not debug and request.get('auth-token') != AUTH_TOKEN:
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

@check_and_route('/register', methods=methods)
def register():
    return "TBD"

@check_and_route('/')
def main():
    return "Hi... nothing much to see here."

@check_and_route('/write/<data>')
def write(data):
    redis.set('test', data)
    return 'OK'

@check_and_route('/read/')
def read():
    return redis.get('test')

@check_and_route('/cf')
@log_tracebacks
def cf_():
    from pprint import pformat
    return ("<pre>"
            +"\n".join(pformat(each)
                       for each in cloudflare.rec_load_all('getiantem.org')
                       if each['display_name'] == 'email')
            +"</pre>")



