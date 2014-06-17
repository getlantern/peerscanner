#!/usr/bin/env bash

./subscriber.py &
gunicorn app:app --preload --worker-class gevent

