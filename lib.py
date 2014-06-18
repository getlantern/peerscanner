from datetime import datetime
import os
import traceback
from functools import wraps

from flask import abort, request
import redis as redis_module
import pyflare


app = None
AUTH_TOKEN = os.getenv('AUTH_TOKEN')
DEBUG = os.getenv('DEBUG') == 'true'
CF_ZONE = 'getiantem.org'
CF_ROUND_ROBIN_SUBDOMAIN = 'peerroundrobin'
OWN_RECID_KEY = 'own_recid'
ROUND_ROBIN_RECID_KEY = 'rr_recid'
DO_CHECK_AUTH = not DEBUG

redis = None
cloudflare = None


def register(name, ip):
    rh = redis.hgetall(redis_key(name))
    if rh:
        refresh_record(name, ip, rh)
    else:
        add_new_record(name, ip)

def unregister(name):
    rh = redis.hgetall(redis_key(name))
    if not rh:
        print "***ERROR: trying to unregister non existent name: %r" % name
        return
    for subdomain, key in [(CF_ROUND_ROBIN_SUBDOMAIN, ROUND_ROBIN_RECID_KEY),
                           (name, OWN_RECID_KEY)]:
        cloudflare.rec_delete(CF_ZONE, rh[key])
    redis.delete(redis_key(name))
    print "record deleted OK"

def refresh_record(name, ip, rh):
    print "refreshing record for %s (%s), redis hash %s" % (name,
                                                            ip,
                                                            rh)
    if rh['ip'] == ip:
        print "IP is alright; leaving cloudflare alone"
    else:
        print "refreshing IP in cloudflare"
        for subdomain, key in [(CF_ROUND_ROBIN_SUBDOMAIN,
                                ROUND_ROBIN_RECID_KEY),
                                (name, OWN_RECID_KEY)]:
            cloudflare.rec_edit(CF_ZONE,
                                'A',
                                rh[key],
                                subdomain,
                                ip,
                                service_mode=1)
    redis.hmset(redis_key(name), {'timestamp': redis_timestamp(),
                                  'ip': ip})
    print "record updated OK"

def add_new_record(name, ip):
    print "adding new record for %s (%s)" % (name, ip)
    hs = {"ip": ip}
    for subdomain, key in [(CF_ROUND_ROBIN_SUBDOMAIN, 'rr_recid'),
                           (name, 'own_recid')]:
        response = cloudflare.rec_new(CF_ZONE,
                                      'A',
                                      subdomain,
                                      ip)
        hs[key] = recid = response['response']['rec']['obj']['rec_id']
        # Set service_mode to "orange cloud".  For some reason we can't do
        # this on rec_new.
        cloudflare.rec_edit(CF_ZONE,
                            'A',
                            recid,
                            subdomain,
                            ip,
                            service_mode=1)
    hs['timestamp'] = redis_timestamp()
    redis.hmset(redis_key(name), hs)
    print "record added OK"

def redis_key(name):
    return 'cf.%s' % name

def redis_timestamp():
    return str(datetime.utcnow())

def login_to_redis():
    global redis
    redis = redis_module.from_url(os.environ['REDISCLOUD_URL'])

def login_to_cloudflare():
    global cloudflare
    cloudflare = pyflare.Pyflare(os.environ['CLOUDFLARE_USER'],
                                 os.environ['CLOUDFLARE_API_KEY'])

def get_param(name):
    return request.args.get(name, request.form.get(name))

def check_auth():
    if DO_CHECK_AUTH and get_param('auth-token') != AUTH_TOKEN:
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
        if DEBUG:
            ret = log_tracebacks(ret)
        return app.route(*args, **kw)(ret)
    return deco
