import os
import time

from flask import abort, Flask, request
import rq

import lib


app = lib.app = Flask(__name__)

if lib.DEBUG:
    methods = ['POST', 'GET']
else:
    methods = ['POST']


lib.login_to_redis()
lib.login_to_cloudflare()
q = rq.Queue(connection=lib.redis)


@lib.check_and_route('/register', methods=methods)
def register():
    name = lib.get_param('name')
    ip = lib.get_param('ip')
    q.enqueue(lib.register, name, ip)
    return "OK"

@lib.check_and_route('/unregister', methods=methods)
def unregister():
    name = lib.get_param('name')
    q.enqueue(lib.unregister, name)
    return "OK"

if lib.DEBUG:

    @lib.check_and_route('/')
    def main():
        return "Hi... nothing much to see here."

    @lib.check_and_route('/write/<data>')
    def write(data):
        redis.set('test', data)
        return 'OK'

    @lib.check_and_route('/read/')
    def read():
        return redis.get('test')

    @lib.check_and_route('/publish/<msg>')
    def publish(msg):
        redis.publish('test', msg)
        return "Sent %s to redis." % msg

    @lib.check_and_route('/rq/<msg>')
    def send_to_rq(msg):
        job = q.enqueue(lib.echo, msg)
        while job.result is None:
            print "Got a None result"
            time.sleep(1)
        return "Sent %r to rq, got %r." % (msg, job.result)

    @lib.check_and_route('/cf')
    def cf_():
        from pprint import pformat
        return ("<pre>"
                +"\n".join(pformat(each)
                           for each in lib.cloudflare.rec_load_all('getiantem.org')
                           if each['display_name'] == 'email')
                +"</pre>")



