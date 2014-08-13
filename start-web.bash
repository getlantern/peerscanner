#!/usr/bin/env bash

python -u ./stale_checker.py &
gunicorn app:app --preload --worker-class gevent --workers $WEB_CONCURRENCY

