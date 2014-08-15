#!/usr/bin/env bash

gunicorn app:app --preload --worker-class gevent --workers $WEB_CONCURRENCY

