#!/usr/bin/env python

import os
import sys


here = os.path.dirname(sys.argv[0])
secret_repo = os.path.join(here, '..', 'too-many-secrets')
pdr_secrets = yaml.load(file(os.path.join(secret_repo,
                                          'peerdnsreg.txt')))
cf_secrets = yaml.load(file(os.path.join(secret_repo,
                                         'cloudflare.txt')))

def setcfg(name, secret):
    os.system("heroku config:set %s=%s" % (name, secret))

setcfg("CLOUDFLARE_USER", cf_secrets['user'])
setcfg("CLOUDFLARE_API_KEY", cf_secrets['api_key'])
setcfg("AUTH_TOKEN", pdr_secrets['auth-token'])
