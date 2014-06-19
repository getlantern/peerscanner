from pprint import pprint
import signal
import time
import traceback

import lib

lib.login_to_redis()


def run():
    print "starting subscriber"
    sub = lib.redis.pubsub()
    sub.subscribe("test", "xyzzy")
    done = [False]
    def handler(signum, frame):
        print "handling SIGTERM"
        redis = lib.login_to_redis()
        done[0] = True
        redis.publish('xyzzy', 'kill')
        print "handled"
    signal.signal(signal.SIGTERM, handler)
    for item in sub.listen():
        if item['type'] == 'message' and item['channel'] == 'test':
            print "got one!"
            pprint(item)
        if item['channel'] == 'xyzzy':
            print "Got a xyzzy", item
        if done[0]:
            print "BYE!"
            break


if __name__ == '__main__':
    try:
        run()
    except:
        traceback.print_exc()
