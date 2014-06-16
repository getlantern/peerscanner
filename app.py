import os

from flask import abort, Flask, request
from pyflare import Pyflare
from redis import Redis


AUTH_TOKEN = os.getenv('AUTH_TOKEN')
app = Flask(__name__)


#@app.route('/')
#def main():
#    return 'Testing 123...'

#@app.route('/init', methods=['POST'])
#def init():
    #if request['auth-token'] != AUTH_TOKEN:
    #    abort(403)

@app.route('/write/<data>')
def write(data):
    r = redis()
    r.set('test', data)
    return 'OK'

@app.route('/read/')
def read():
    r = redis()
    return r.get('test')

@app.route('/cf')
def cf_():
    cf = cloudflare()
    from pprint import pformat
    return "<br>".join(pformat(each.__dict__)
                       for each in cf.rec_get_all('getiantem.org'))

def redis():
    url = urlparse.urlparse(os.environ['REDISCLOUD_URL'])
    return Redis(host=url.hostname, port=url.port, password=url.password)

def cloudflare():
    return Pyflare(os.environ['CLOUDFLARE_USER'],
                   os.environ['CLOUDFLARE_API_KEY'])
