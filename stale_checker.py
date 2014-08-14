from pprint import pprint
import signal
import time
import traceback

import lib

MINUTE = 60
SLEEP_TIME = 10 #1 * MINUTE


def run():
    print "starting expirer"
    class Done(Exception):
        pass
    def handler(signum, frame):
        print "handling SIGTERM"
        raise Done
    signal.signal(signal.SIGTERM, handler)
    try:
        lib.login_to_redis()
        lib.login_to_cloudflare()
        while True:
            lib.remove_stale_entries()
            time.sleep(SLEEP_TIME)
    except Done:
        print "Caught SIGTERM; bye!"


if __name__ == '__main__':
    try:
        run()
    except:
        traceback.print_exc()
