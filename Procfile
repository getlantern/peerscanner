#!/usr/bin/env bash

web: ./start-web.bash
worker: python -u ./rq_worker.py
clock: python -u ./stale_checker.py
