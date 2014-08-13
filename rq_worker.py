import os

from rq import Worker, Queue, Connection

import lib

def exception_handler(job, exc_type, exc_value, traceback):
	print ("ERROR job: %s, exc_type: %s, exc_value %s, traceback: %s" % (job, exc_type, exc_value, traceback))

if __name__ == '__main__':
    lib.login_to_redis()
    lib.login_to_cloudflare()
    with Connection(lib.redis):
        worker = Worker([Queue()], exc_handler=exception_handler)
        worker.work()
