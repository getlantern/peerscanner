#!/usr/bin/env python

import sys
import time

import redis as redis_


def subscribe():
    redis = redis_.from_url(os.environ['REDISCLOUD_URL'])


if __name__ == '__main__':
    for x in range(10):
        print "DONG", x
        sys.stdout.flush()
        time.sleep(2)
