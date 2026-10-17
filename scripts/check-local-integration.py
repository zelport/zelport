#!/usr/bin/env python3
"""Linux process acceptance for a supplied Zelport App binary and plugin helper.

Run in an isolated container as an unprivileged user. Uses only temporary files.
"""
import http.client
import json
import os
from pathlib import Path
import socket
import subprocess
import sys
import tempfile
import time

binary, helper = map(os.path.abspath, sys.argv[1:])
assert sys.platform == 'linux' and os.geteuid() != 0

def free_port():
    with socket.socket() as sock:
        sock.bind(('127.0.0.1', 0))
        return sock.getsockname()[1]

def get(port):
    c = http.client.HTTPConnection('127.0.0.1', port, timeout=.3)
    try:
        c.request('GET', '/api/integration/v1/state', headers={'Host':f'localhost:{port}'})
        r=c.getresponse()
        assert r.status==200
        return json.loads(r.read())
    finally: c.close()

with tempfile.TemporaryDirectory() as directory:
    root=Path(directory)
    env={**os.environ,'HOME':directory,'XDG_STATE_HOME':str(root/'state'),'XDG_CONFIG_HOME':str(root/'config')}
    env.pop('XDG_RUNTIME_DIR',None)
    hint=root/'state/zelport/endpoint.json'
    def start(port):
        log=open(root/'zelport.log','w+')
        p=subprocess.Popen([binary,'--port',str(port),'--config',str(root/'custom/config.json')],env=env,stdout=log,stderr=log)
        try:
            for _ in range(100):
                if p.poll() is not None: raise AssertionError('Zelport App exited')
                try:
                    result=get(port)
                    if port!=8080 and not hint.exists(): raise OSError('hint not ready')
                    return p, log, result
                except OSError: time.sleep(.03)
            raise AssertionError('startup timed out')
        except BaseException:
            p.terminate();p.wait(timeout=5);log.seek(0);print(log.read());log.close();raise
    def stop(p, log):
        p.terminate(); assert p.wait(timeout=5)==0
        log.close()
    def poll():
        result=subprocess.run([sys.executable,helper],env=env,capture_output=True,text=True,timeout=3,check=True)
        assert not result.stderr,result.stderr
        return json.loads(result.stdout)
    for port in (free_port(), free_port()):
        p,log,data=start(port)
        try:
            assert data['product']=='zelport' and 'instance_id' not in data
            assert json.loads(hint.read_text())=={'port':port}
            assert hint.stat().st_mode & 0o777==0o600
            result=poll();assert result['ok'],result
            assert result['state']['admin_url']==f'http://localhost:{port}'
        finally: stop(p,log)
        assert hint.exists()
    p,log,_=start(8080)
    try:
        hint.unlink()
        assert poll()['ok'], 'missing hint did not use 8080'
        hint.write_text(json.dumps({'port':free_port()}))
        assert poll()['ok'], 'stale hint did not fall back to 8080'
    finally: stop(p,log)
    # Recording failure must not prevent serving the API.
    blocker=root/'blocked';blocker.write_text('not a directory')
    env['XDG_STATE_HOME']=str(blocker)
    p,log,_=start(8080)
    try:
        assert poll()['ok']
        log.flush();log.seek(0)
        assert 'cannot record local endpoint' in log.read()
    finally: stop(p,log)
print('PASS: startup, restart, port change, no runtime directory, fallback and nonfatal state write failure')
