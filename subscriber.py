#!/usr/bin/env python

import redis as redis_
from pyflare import Pyflare
import time

def subscribe():
    redis = redis_.from_url(os.environ['REDISCLOUD_URL'])

if __name__ == '__main__':
    for x in range(10):
        print "DONG", x
        time.sleep(2)
