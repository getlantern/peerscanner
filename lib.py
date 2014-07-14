from contextlib import contextmanager
from datetime import datetime
import os
import time
import traceback
from functools import wraps

from fastly import connect as connect_to_fastly, FastlyError
from flask import abort, request
import pyflare
import redis as redis_module


app = None
AUTH_TOKEN = os.getenv('AUTH_TOKEN')
DEBUG = os.getenv('DEBUG') == 'true'
DOMAIN = 'getiantem.org'
CF_ROUND_ROBIN_SUBDOMAIN = 'peerroundrobin'
OWN_RECID_KEY = 'own_recid'
ROUND_ROBIN_RECID_KEY = 'rr_recid'
DO_CHECK_AUTH = False  # clients are registering directly in this MVP
NAME_BY_TIMESTAMP_KEY = 'name_by_ts'
MINUTE = 60
CHECK_STALE_PERIOD = 1 * MINUTE
STALE_TIME = 5 * MINUTE
DIRECTOR_NAME = "PeerAutoDirector"
DIRECTOR_QUORUM_PERCENTAGE = 1
DIRECTOR_RETRIES = 10
FP_PREFIX = "fp-"

cloudflare = None
fastly = None
redis = None


def register(name, ip, port):
    int_port = int(port)
    if int_port <= 0 or int_port > 65535:
        print "***ERROR: trying to register with invalid port: %s" % int_port
        return

    rh = redis.hgetall(rh_key(name))
    if rh:
        update_fastly_backend(name, ip, port, rh)
    else:
        create_fastly_backend(name, ip, port)

def unregister(name):
    rh = redis.hgetall(rh_key(name))
    if not rh:
        print "***ERROR: trying to unregister non existent name: %r" % name
        return
    delete_fastly_backend(name)

def create_fastly_backend(name, ip, port):
    print "registering fastly backend %r at %s:%s" % (name, ip, port)
    rh = {'ip': ip, 'port': port}
    svcid = fastly_svcid()
    with fastly_version() as version:
        try:
            fastly.create_condition(svcid,
                                    version,
                                    name,
                                    'REQUEST',
                                    'req.http.host == "%s.%s"' % (name,
                                                                  DOMAIN))
        except FastlyError:
            # Maybe we have already created this condition, probably in
            # a previous failed attempt to create this backend.
            print "Error while trying to create condition; ignoring."
            traceback.print_exc()
        fastly.create_backend(svcid,
                              version,
                              name,
                              ip,
                              port=port,
                              auto_loadbalance=True,
                              weight=100,
                              error_threshold=10,
                              request_condition=name,
                              healthcheck="HEAD OK",
                              max_conn=2000,
                              connect_timeout=10000,
                              first_byte_timeout=30000,
                              between_bytes_timeout=80000,
                              comment="added by peerdnsreg")

        fastly.create_director_backend(svcid, version, DIRECTOR_NAME, name)

    rh['last_updated'] = redis_datetime()
    with transaction() as rt:
        rt.hmset(rh_key(name), rh)
        rt.zadd(NAME_BY_TIMESTAMP_KEY, name, redis_timestamp())
    print "backend created OK"

def update_fastly_backend(name, ip, port, rh):
    print "updating fastly backend %r at %s:%s, %r" % (name, ip, port, rh)
    if rh['ip'] == ip and rh['port'] == port:
        print "backend is up-to-date; leaving Fastly alone"
    else:
        print "backend needs updating"
        with fastly_version() as version:
            fastly.update_backend(fastly_svcid(),
                                  version,
                                  name,
                                  address=ip,
                                  port=port)
    with transaction() as rt:
        rt.hmset(rh_key(name), {'last_updated': redis_datetime(),
                                'ip': ip,
                                'port': port})
        rt.zadd(NAME_BY_TIMESTAMP_KEY, name, redis_timestamp())
    print "backend updated OK"

def delete_fastly_backend(name):
    svcid = fastly_svcid()
    with fastly_version() as version:
        fastly.delete_backend(svcid, version, name)
        fastly.delete_condition(svcid, version, name)
    with transaction() as rt:
        rt.delete(rh_key(name))
        rt.zrem(NAME_BY_TIMESTAMP_KEY, name)
    print "record deleted OK"

def fastly_svcid():
    return os.environ['FASTLY_SERVICE_ID']

@contextmanager
def fastly_version():
    edit_version = int(os.environ['FASTLY_VERSION'])
    update_load_balancer(edit_version)
    yield edit_version
    new_version = fastly.clone_version(fastly_svcid(), edit_version)
    fastly.activate_version(fastly_svcid(), new_version.number)

def update_load_balancer(version):
    svcid = fastly_svcid()
    try:
        fastly.create_director(svcid,
                               version,
                               DIRECTOR_NAME,
                               quorum=DIRECTOR_QUORUM_PERCENTAGE,
                               retries=DIRECTOR_RETRIES)
    except:
        # If we couldn't create the director, that means we probably already had
        # it, don't worry
        pass

    # Create fallback proxy
    name = "sp1"
    # TODO: DRY violation with create_fastly_backend
    try:
        fastly.create_condition(svcid,
                                    version,
                                    name,
                                    'REQUEST',
                                    'req.http.host == "%s.%s"' % (name,
                                                                  DOMAIN))
        fastly.create_backend(svcid,
                              version,
                              name,
                              "128.199.178.240",
                              port=80,
                              auto_loadbalance=True,
                              weight=1000,
                              error_threshold=200,
                              request_condition=name,
                              healthcheck="HEAD OK",
                              max_conn=2000,
                              connect_timeout=10000,
                              first_byte_timeout=30000,
                              between_bytes_timeout=80000,
                              comment="fallback added by peerdnsreg"
        
    except:
        pass
    # Add all fallback proxy backends to director (fallback proxies must have
    # names starting with fp-)
    fastly_version = fastly.get_version(svcid, version)
    for backend in fastly_version.backends:
        if backend.startswith(FP_PREFIX):
            try:
                fastly.create_director_backend(svcid, 
                                               version,
                                               DIRECTOR_NAME,
                                               backend)
            except:
                # If we couldn't create the backend, that probably means we
                # already had it, don't worry
                pass


def remove_stale_entries():
    cutoff = time.time() - STALE_TIME
    for name in redis.zrangebyscore(NAME_BY_TIMESTAMP_KEY,
                                    '-inf',
                                    cutoff):
        try:
            unregister(name)
        except:
            print "Exception unregistering %s:" % name
            traceback.print_exc()

# Cloudflare stuff commented out for future reference.  Some refactoring will
# be in order when we want to bring this back.
#
#def add_cf_record(name, ip):
#    print "adding new record for %s (%s)" % (name, ip)
#    rh = {"ip": ip}
#    for subdomain, key in [(CF_ROUND_ROBIN_SUBDOMAIN, 'rr_recid'),
#                           (name, 'own_recid')]:
#        response = cloudflare.rec_new(DOMAIN,
#                                      'A',
#                                      subdomain,
#                                      ip)
#        rh[key] = recid = response['response']['rec']['obj']['rec_id']
#        # Set service_mode to "orange cloud".  For some reason we can't do
#        # this on rec_new.
#        cloudflare.rec_edit(DOMAIN,
#                            'A',
#                            recid,
#                            subdomain,
#                            ip,
#                            service_mode=1)
#    rh['last_updated'] = redis_datetime()
#    with transaction() as rt:
#        rt.hmset(rh_key(name), rh)
#        rt.zadd(NAME_BY_TIMESTAMP_KEY, name, redis_timestamp())
#    print "record added OK"
#
#def refresh_cf_record(name, ip, rh):
#    print "refreshing record for %s (%s), redis hash %s" % (name, ip, rh)
#    if rh['ip'] == ip:
#        print "IP is alright; leaving cloudflare alone"
#    else:
#        print "refreshing IP in cloudflare"
#        for subdomain, key in [(CF_ROUND_ROBIN_SUBDOMAIN,
#                                ROUND_ROBIN_RECID_KEY),
#                                (name, OWN_RECID_KEY)]:
#            cloudflare.rec_edit(DOMAIN,
#                                'A',
#                                rh[key],
#                                subdomain,
#                                ip,
#                                service_mode=1)
#    with transaction() as rt:
#        rt.hmset(rh_key(name), {'last_updated': redis_datetime(), 'ip': ip})
#        rt.zadd(NAME_BY_TIMESTAMP_KEY, name, redis_timestamp())
#    print "record updated OK"
#
#def remove_cf_record(name, rh):
#    for subdomain, key in [(CF_ROUND_ROBIN_SUBDOMAIN, ROUND_ROBIN_RECID_KEY),
#                           (name, OWN_RECID_KEY)]:
#        cloudflare.rec_delete(DOMAIN, rh[key])
#    with transaction() as rt:
#        rt.delete(rh_key(name))
#        rt.zrem(NAME_BY_TIMESTAMP_KEY, name)
#    print "record deleted OK"

@contextmanager
def transaction():
    txn = redis.pipeline(transaction=True)
    yield txn
    txn.execute()

def rh_key(name):
    return 'cf:%s' % name

def redis_datetime():
    "Human-readable version, for debugging."
    return str(datetime.utcnow())

def redis_timestamp():
    "Seconds since epoch, used for sorting."
    return time.time()

def login_to_redis():
    global redis
    redis = redis_module.from_url(os.environ['REDISCLOUD_URL'])

def login_to_fastly():
    global fastly
    fastly = connect_to_fastly(os.environ['FASTLY_API_KEY'])
    fastly.login(os.environ['FASTLY_USER'],
                 os.environ['FASTLY_PASSWORD'])

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
