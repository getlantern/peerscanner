#!/usr/bin/env python

# Assumes a copy of the too-many-secrets repo as a sibling of the directory
# where this is contained.
#
# To import this config for running locally:
#
#    heroku plugins:install git://github.com/ddollar/heroku-config.git
#    heroku config:pull --overwrite
#
# And then you may want to change DEBUG to 'true'.

import os
import sys

import yaml


def load_secrets(filename):
    here = os.path.dirname(sys.argv[0])
    secret_repo = os.path.join(here, '..', 'too-many-secrets')
    return yaml.load(file(os.path.join(secret_repo, filename)))

pdr_secrets = load_secrets('peerdnsreg.txt')
cf_secrets = load_secrets('cloudflare.txt')
fastly_secrets = load_secrets('fastly.com.md')

def setcfg(name, secret):
    os.system("heroku config:set %s='%s'" % (name, secret))

setcfg("CLOUDFLARE_USER", cf_secrets['user'])
setcfg("CLOUDFLARE_API_KEY", cf_secrets['api_key'])
setcfg("FASTLY_USER", fastly_secrets['user'])
setcfg("FASTLY_PASSWORD", fastly_secrets['password'])
setcfg("FASTLY_API_KEY", fastly_secrets['api_key'])
setcfg("FASTLY_SERVICE_ID", '11yqoXJrAAGxPiC07v3q9Z')
setcfg("FASTLY_VERSION", '32')
setcfg("AUTH_TOKEN", pdr_secrets['auth_token'])
setcfg("WEB_CONCURRENCY", 2)
setcfg("DEBUG", 'false')
