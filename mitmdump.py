# This is a mitmdump script that logs all requests made to Clubhouse APIs.
#
# Usage:
# 1. Configure iptables to intercept all http and https requests and send
#    them to mitmdump:
#    -A PREROUTING -i eth1 -p tcp -m tcp --dport 80 -j REDIRECT --to-ports 8080
#    -A PREROUTING -i eth1 -p tcp -m tcp --dport 443 -j REDIRECT --to-ports 8080
# 2. Run mitmdump in a transparent mode with this script:
#    mitmdump --mode transparent --listen-port 8080 --listen-host 0.0.0.0 --showhost -s mitmdump.py

import datetime
import json
import mitmproxy.http

class Logger:
    def response(self, flow: mitmproxy.http.HTTPFlow):
        if not flow.request.headers.get('host', '') in ['clubhouse.pubnub.com', 'clubhouse.pubnubapi.com', 'www.clubhouseapi.com']:
            return
        log = {
            'ts': datetime.datetime.now().timestamp(),
            'request': {
                'method': flow.request.method,
                'headers': {k.decode(): v.decode() for k, v in flow.request.headers.fields},
                'url': flow.request.url,
                'text': flow.request.text,
            },
            'response': {
                'status_code': flow.response.status_code,
                'headers': {k.decode(): v.decode() for k, v in flow.response.headers.fields},
                'cookies': {k: v[0] for k, v in flow.response.cookies.items()},
                'text': flow.response.text,
            },
        }
        with open('/var/log/mitmproxy.log', 'a+') as f:
            f.write(json.dumps(log) + '\n')

addons = [Logger()]
