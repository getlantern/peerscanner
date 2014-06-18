import os

from rq import Worker, Queue, Connection

import lib


listen = ['default']

if __name__ == '__main__':
    with Connection(lib.login_to_redis()):
        worker = Worker(map(Queue, listen))
        worker.work()
