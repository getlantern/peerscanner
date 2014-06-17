from pprint import pprint
import time

import lib

redis = lib.login_to_redis()
#cloudflare = lib.login_to_cloudflare()


def run():
    sub = redis.pubsub()
    sub.subscribe("test")
    for item in sub.listen():
        # Ignore 'subscribe' events
        if item['type'] == 'message':
            print "got one!"
            pprint(item)


if __name__ == '__main__':
    run()
