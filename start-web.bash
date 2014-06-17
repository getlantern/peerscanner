#!/usr/bin/env bash

python -u ./subscriber.py &
gunicorn app:app --preload --worker-class gevent

