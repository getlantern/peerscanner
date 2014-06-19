from pprint import pprint
import signal
import time

import lib

lib.login_to_redis()


def run():
    print "starting subscriber"
    sub = lib.redis.pubsub()
    sub.subscribe("test")
    done = [False]
    def handler(signum, frame):
        print "handling SIGTERM"
        done[0] = True
        sub.unsubscribe()
    signal.signal(signal.SIGTERM, handler)
    for item in sub.listen():
        if item['type'] == 'unsubscribe':
            print "Got unsubscribe."
        if item['type'] == 'message':
            print "got one!"
            pprint(item)
        if done[0]:
            print "BYE!"
            break


if __name__ == '__main__':
    run()
