#!/usr/bin/env bash

python -u ./rq_worker.py &
python -u ./stale_checker.py

