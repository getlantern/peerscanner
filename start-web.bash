#!/usr/bin/env bash

python -u ./subscriber.py &
python -u ./rq_worker.py &
gunicorn app:app --preload --worker-class gevent --workers $WEB_CONCURRENCY

