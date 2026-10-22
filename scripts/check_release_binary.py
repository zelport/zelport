#!/usr/bin/env python3
"""Verify and execute an already-packaged Zelport App release binary."""

import argparse
import hashlib
import hmac
import http.client
import http.server
import importlib.util
import json
import os
from pathlib import Path
import secrets
import socket
import subprocess
import tarfile
import tempfile
import threading
import time


_release_spec = importlib.util.spec_from_file_location(
    'release', Path(__file__).with_name('release.py'))
release = importlib.util.module_from_spec(_release_spec)
_release_spec.loader.exec_module(release)

EXPECTED_MEMBERS = {
    'zelport': 0o755,
    'LICENSE': 0o644,
    'THIRD_PARTY_NOTICES.txt': 0o644,
}
EXPECTED_MODULES = {
    'built_in_relay',
    'local_ports',
    'ports_over_relay',
    'ports_over_ssh',
    'remote_http',
}


def _expected_checksum(checksums: Path, archive_name: str) -> str:
    matches = []
    for line in checksums.read_text(encoding='utf-8').splitlines():
        fields = line.split()
        if len(fields) == 2 and fields[1] == archive_name:
            matches.append(fields[0])
    if len(matches) != 1:
        raise ValueError(f'SHA256SUMS must contain exactly one entry for {archive_name}')
    return matches[0]


def verify_archive(bundle: Path, version: str, platform: str,
                   destination: Path) -> Path:
    """Verify one release archive and safely extract its three regular files."""
    release.validate_version(version)
    if platform not in release.PLATFORMS:
        raise ValueError(f'unknown release platform: {platform}')
    bundle = Path(bundle)
    archive = bundle / f'zelport_{version}_{platform}.tar.gz'
    checksums = bundle / 'SHA256SUMS'
    if not checksums.is_file():
        raise FileNotFoundError(f'SHA256SUMS not found in {bundle}')
    expected_digest = _expected_checksum(checksums, archive.name)
    actual_digest = hashlib.sha256(archive.read_bytes()).hexdigest()
    if not hmac.compare_digest(actual_digest, expected_digest):
        raise ValueError(f'checksum mismatch for {archive.name}')

    with tarfile.open(archive, 'r:gz') as source:
        members = source.getmembers()
        names = [member.name for member in members]
        if len(members) != len(EXPECTED_MEMBERS) or set(names) != set(EXPECTED_MEMBERS):
            raise ValueError(f'unexpected archive members: {names}')
        for member in members:
            if not member.isreg():
                raise ValueError(f'archive member is not a regular file: {member.name}')
            expected_mode = EXPECTED_MEMBERS[member.name]
            if member.mode != expected_mode:
                raise ValueError(
                    f'archive member mode for {member.name} is '
                    f'{member.mode:o}, expected {expected_mode:o}')

        destination = Path(destination)
        destination.mkdir(parents=True, exist_ok=False)
        for member in members:
            content = source.extractfile(member)
            if content is None:
                raise ValueError(f'cannot read archive member: {member.name}')
            target = destination / member.name
            target.write_bytes(content.read())
            target.chmod(EXPECTED_MEMBERS[member.name])
    return destination / 'zelport'


def _log_text(path: Path) -> str:
    try:
        return path.read_text(encoding='utf-8', errors='replace')
    except FileNotFoundError:
        return ''


def _force_stop(process: subprocess.Popen) -> None:
    if process.poll() is not None:
        return
    process.terminate()
    try:
        process.wait(timeout=1)
    except subprocess.TimeoutExpired:
        process.kill()
        process.wait(timeout=1)


def terminate_process(process: subprocess.Popen, log_path: Path,
                      timeout: float = 5) -> None:
    """Request graceful shutdown, killing and diagnosing an unresponsive child."""
    if process.poll() is None:
        process.terminate()
    try:
        status = process.wait(timeout=timeout)
    except subprocess.TimeoutExpired as error:
        process.kill()
        process.wait(timeout=1)
        raise AssertionError(
            f'Zelport App ignored SIGTERM\nZelport log:\n{_log_text(log_path)}') from error
    if status != 0:
        raise AssertionError(
            f'Zelport App exited with status {status}\n'
            f'Zelport log:\n{_log_text(log_path)}')


class ProcessOwner:
    """Own child processes and their logs for one acceptance scenario."""

    def __init__(self):
        self.entries = []

    def start(self, args, env, log_path: Path) -> subprocess.Popen:
        log_path = Path(log_path)
        log = log_path.open('w')
        try:
            process = subprocess.Popen(
                args, env=env, stdout=log, stderr=subprocess.STDOUT)
        except BaseException:
            log.close()
            raise
        self.entries.append((process, log, log_path))
        return process

    def close(self, timeout: float = 5) -> None:
        errors = []
        for process, log, log_path in reversed(self.entries):
            try:
                terminate_process(process, log_path, timeout)
            except AssertionError as error:
                errors.append(str(error))
            finally:
                log.close()
        self.entries.clear()
        if errors:
            raise AssertionError('\n'.join(errors))

    def force_close(self) -> None:
        for process, log, _ in reversed(self.entries):
            try:
                _force_stop(process)
            finally:
                log.close()
        self.entries.clear()

    def diagnostics(self) -> str:
        return '\n'.join(
            f'{path.name}:\n{_log_text(path)}' for _, _, path in self.entries)


def wait_for(probe, ready, processes, *, timeout: float,
             description: str):
    """Poll a stateful condition while ensuring owned processes remain alive."""
    deadline = time.monotonic() + timeout
    last_state = None
    last_error = None
    while time.monotonic() < deadline:
        for process in processes:
            if process.poll() is not None:
                raise AssertionError(
                    f'{description} stopped because process {process.pid} '
                    f'exited with status {process.returncode}')
        try:
            last_state = probe()
            if ready(last_state):
                return last_state
        except (OSError, ValueError, KeyError, AssertionError) as error:
            last_error = error
        time.sleep(.02)
    detail = f'last state: {last_state!r}'
    if last_error is not None:
        detail += f', last error: {last_error}'
    raise AssertionError(f'{description} timed out; {detail}')


def _free_port() -> int:
    with socket.socket(socket.AF_INET, socket.SOCK_STREAM) as listener:
        listener.bind(('127.0.0.1', 0))
        return listener.getsockname()[1]


def _has_ipv6_loopback() -> bool:
    if not socket.has_ipv6:
        return False
    try:
        with socket.socket(socket.AF_INET6, socket.SOCK_STREAM) as listener:
            listener.bind(('::1', 0))
        return True
    except OSError:
        return False


def _request(port: int, path: str, *, host: str | None = None,
             method: str = 'GET', payload=None, csrf: str | None = None):
    connection = http.client.HTTPConnection('127.0.0.1', port, timeout=2)
    try:
        headers = {'Host': host or f'localhost:{port}'}
        body = None
        if payload is not None:
            headers.update({
                'Content-Type': 'application/json',
                'Origin': f'http://localhost:{port}',
                'X-CSRF-Token': csrf or '',
            })
            body = json.dumps(payload)
        connection.request(method, path, body=body, headers=headers)
        response = connection.getresponse()
        return response.status, response.read(), dict(response.headers)
    finally:
        connection.close()


def _state(port: int):
    status, body, _ = _request(port, '/api/state')
    if status != 200:
        raise AssertionError(f'/api/state returned {status}: {body!r}')
    return json.loads(body)


def _mutate(port: int, path: str, method: str, payload):
    for _ in range(40):
        state = _state(port)
        body = dict(payload)
        body['revision'] = state['revision']
        status, response, _ = _request(
            port, path, method=method, payload=body, csrf=state['csrf'])
        if status == 409:
            continue
        if status not in (200, 201):
            raise AssertionError(f'{method} {path} returned {status}: {response!r}')
        return json.loads(response)
    raise AssertionError(f'{method} {path} remained conflicted')


def _wait_for_state(process: subprocess.Popen, port: int,
                    timeout: float):
    deadline = time.monotonic() + timeout
    last_error = None
    while time.monotonic() < deadline:
        if process.poll() is not None:
            raise AssertionError(
                f'Zelport App exited before startup with status {process.returncode}')
        try:
            return _state(port)
        except (OSError, ValueError, AssertionError) as error:
            last_error = error
            time.sleep(.02)
    raise AssertionError(f'Zelport App startup timed out: {last_error}')


def assert_removed_modules_absent(state) -> bool:
    """Require removed feature modules to be absent from the standard build."""
    unexpected = {'containers', 'docker_sandboxes'} & state.get('states', {}).keys()
    if unexpected:
        raise AssertionError(f'removed module state is present: {sorted(unexpected)}')
    return True


class _Backend(http.server.BaseHTTPRequestHandler):
    def do_GET(self):
        body = b'native release backend'
        self.send_response(207)
        self.send_header('Content-Type', 'text/plain')
        self.send_header('Content-Length', str(len(body)))
        self.end_headers()
        self.wfile.write(body)

    def log_message(self, *_):
        pass


def run_app_acceptance(binary: Path, version: str, platform: str, *,
                       startup_timeout: float = 10,
                       shutdown_timeout: float = 5) -> None:
    """Exercise one supplied native Zelport App executable."""
    release.validate_version(version)
    if platform not in release.PLATFORMS:
        raise ValueError(f'unknown release platform: {platform}')
    binary = Path(binary).resolve()
    reported = subprocess.run(
        [str(binary), '--version'], capture_output=True, text=True, timeout=5)
    expected_prefix = f'zelport {version} (built '
    if (reported.returncode != 0 or not reported.stdout.startswith(expected_prefix)
            or not reported.stdout.rstrip().endswith(')') or reported.stderr):
        raise AssertionError(
            f'Zelport version output does not report version {version}: '
            f'stdout={reported.stdout!r}, stderr={reported.stderr!r}')

    if hasattr(os, 'geteuid') and os.geteuid() == 0:
        raise AssertionError('native acceptance must run as an ordinary user')

    with tempfile.TemporaryDirectory(prefix='zelport-release-acceptance-') as tmp:
        work = Path(tmp)
        empty_path = work / 'empty-path'
        empty_path.mkdir()
        config = work / 'config.json'
        log_path = work / 'zelport.log'
        backend = http.server.ThreadingHTTPServer(('127.0.0.1', 0), _Backend)
        backend_thread = threading.Thread(target=backend.serve_forever, daemon=True)
        backend_thread.start()
        app_port = _free_port()
        route_id = '82958650-fb63-43c1-903b-f3b6f89b1667'
        config.write_text(json.dumps({
            'version': 1,
            'revision': 1,
            'modules': {},
            'routes': [{
                'id': route_id,
                'module': 'local_ports',
                'domain': 'native-release.localhost',
                'settings': {'aux_port': backend.server_port},
            }],
        }), encoding='utf-8')
        env = {
            'HOME': str(work / 'home'),
            'PATH': str(empty_path),
            'XDG_CONFIG_HOME': str(work / 'xdg-config'),
            'XDG_STATE_HOME': str(work / 'xdg-state'),
        }
        process = None
        try:
            with log_path.open('w') as log:
                process = subprocess.Popen(
                    [str(binary), '--port', str(app_port), '--config', str(config)],
                    env=env, stdout=log, stderr=subprocess.STDOUT)
                try:
                    state = _wait_for_state(process, app_port, startup_timeout)
                    runtime = state['runtime']
                    if runtime['version'] != version or runtime['platform'] != platform.replace('_', '/'):
                        raise AssertionError(f'unexpected runtime identity: {runtime}')
                    listeners = runtime['listeners']
                    if f'127.0.0.1:{app_port}' not in listeners:
                        raise AssertionError(f'IPv4 loopback listener missing: {listeners}')
                    if _has_ipv6_loopback() and f'[::1]:{app_port}' not in listeners:
                        raise AssertionError(f'IPv6 loopback listener missing: {listeners}')
                    modules = {module['id'] for module in state['modules']}
                    if modules != EXPECTED_MODULES:
                        raise AssertionError(
                            f'standard module IDs are {sorted(modules)}, '
                            f'want {sorted(EXPECTED_MODULES)}')
                    assert_removed_modules_absent(state)

                    status, body, headers = _request(
                        app_port, '/probe?q=1',
                        host=f'native-release.localhost:{app_port}')
                    if status != 207 or body != b'native release backend':
                        raise AssertionError(
                            f'reverse proxy returned {status}, {body!r}, {headers!r}')

                    status, body, _ = _request(app_port, '/api/integration/v1/state')
                    integration = json.loads(body)
                    services = integration.get('services', [])
                    if (status != 200 or integration.get('product') != 'zelport'
                            or integration.get('api_version') != 1
                            or integration.get('version') != version
                            or len(services) != 1
                            or services[0].get('id') != route_id
                            or services[0].get('source_id') != 'local_ports'):
                        raise AssertionError(
                            f'unexpected Local Integration response: {status}, {integration}')
                    terminate_process(process, log_path, shutdown_timeout)
                    process = None
                except Exception as error:
                    if process is not None:
                        _force_stop(process)
                        process = None
                    raise AssertionError(
                        f'{error}\nZelport log:\n{_log_text(log_path)}') from error
        finally:
            if process is not None:
                _force_stop(process)
            backend.shutdown()
            backend.server_close()
            backend_thread.join(timeout=2)


def _app_environment(work: Path) -> dict[str, str]:
    empty_path = work / 'empty-path'
    empty_path.mkdir(exist_ok=True)
    return {
        'HOME': str(work / 'home'),
        'PATH': str(empty_path),
        'XDG_CONFIG_HOME': str(work / 'xdg-config'),
        'XDG_STATE_HOME': str(work / 'xdg-state'),
    }


def _write_config(path: Path, routes=None) -> None:
    path.write_text(json.dumps({
        'version': 1,
        'revision': 1,
        'modules': {},
        'routes': routes or [],
    }), encoding='utf-8')


def run_relay_acceptance(binary: Path, *, timeout: float = 15) -> None:
    """Prove Ports over Relay data flow through the supplied App executable."""
    binary = Path(binary).resolve()
    with tempfile.TemporaryDirectory(prefix='zelport-release-relay-') as tmp:
        work = Path(tmp)
        provider_work = work / 'provider'
        consumer_work = work / 'consumer'
        provider_work.mkdir()
        consumer_work.mkdir()
        owner = ProcessOwner()
        backend = None
        backend_thread = None
        try:
            backend = http.server.ThreadingHTTPServer(('127.0.0.1', 0), _Backend)
            backend_thread = threading.Thread(
                target=backend.serve_forever, daemon=True)
            backend_thread.start()
            provider_port, consumer_port, relay_port = (
                _free_port(), _free_port(), _free_port())
            source_id = 'd6ecf6f4-f393-4c67-b8db-66c842567d0e'
            provider_config = provider_work / 'config.json'
            consumer_config = consumer_work / 'config.json'
            _write_config(provider_config, [{
                'id': source_id,
                'module': 'local_ports',
                'domain': 'relay-source.localhost',
                'settings': {'aux_port': backend.server_port},
            }])
            _write_config(consumer_config)
            provider = owner.start(
                [str(binary), '--port', str(provider_port),
                 '--config', str(provider_config)],
                _app_environment(provider_work), provider_work / 'zelport.log')
            consumer = owner.start(
                [str(binary), '--port', str(consumer_port),
                 '--config', str(consumer_config)],
                _app_environment(consumer_work), consumer_work / 'zelport.log')
            _wait_for_state(provider, provider_port, timeout)
            _wait_for_state(consumer, consumer_port, timeout)
            _mutate(provider_port, '/api/modules/built_in_relay/settings', 'PUT', {
                'enabled': True,
                'port': relay_port,
                'bind_address': '127.0.0.1',
                'require_access_token': False,
            })
            relay_url = f'ws://127.0.0.1:{relay_port}'
            group = secrets.token_urlsafe(32)
            common = {
                'enabled': True,
                'relay_url': relay_url,
                'access_token': '',
                'group_token': group,
            }
            _mutate(provider_port, '/api/modules/ports_over_relay/settings',
                    'PUT', {**common, 'publish': True})
            _mutate(consumer_port, '/api/modules/ports_over_relay/settings',
                    'PUT', {**common, 'publish': False})

            def offered_record():
                status = _state(consumer_port)['states']['ports_over_relay']['data']['status']
                if not status.get('connected'):
                    return {'status': status, 'record': None}
                record = next((
                    record for record in status.get('records', [])
                    if any(service.get('id') == source_id
                           for service in record.get('services', []))
                ), None)
                return {'status': status, 'record': record}

            offered = wait_for(
                offered_record, lambda value: value['record'] is not None,
                [provider, consumer], timeout=timeout,
                description='relay service offer')
            record = offered['record']
            _mutate(consumer_port, '/api/routes', 'POST', {'route': {
                'module': 'ports_over_relay',
                'domain': 'relay-import.localhost',
                'settings': {
                    'group': record['group'],
                    'device_id': record['device_id'],
                    'service_id': source_id,
                    'provider_key': record['public_key'],
                    'rewrite_origin': False,
                },
            }})

            result = wait_for(
                lambda: _request(
                    consumer_port, '/relay-probe',
                    host=f'relay-import.localhost:{consumer_port}')[:2],
                lambda value: value == (207, b'native release backend'),
                [provider, consumer], timeout=timeout,
                description='Ports over Relay request')
            if result != (207, b'native release backend'):
                raise AssertionError(f'unexpected relay response: {result!r}')
            owner.close()
        except Exception as error:
            diagnostics = owner.diagnostics()
            owner.force_close()
            raise AssertionError(f'{error}\nRelay process logs:\n{diagnostics}') from error
        finally:
            owner.force_close()
            if backend is not None:
                backend.shutdown()
                backend.server_close()
            if backend_thread is not None:
                backend_thread.join(timeout=2)


def main() -> None:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--bundle', type=Path, required=True)
    parser.add_argument('--version', required=True)
    parser.add_argument('--platform', choices=release.PLATFORMS, required=True)
    args = parser.parse_args()
    with tempfile.TemporaryDirectory(prefix='zelport-release-extract-') as tmp:
        binary = verify_archive(
            args.bundle, args.version, args.platform, Path(tmp) / 'archive')
        run_app_acceptance(binary, args.version, args.platform)
        run_relay_acceptance(binary)
    print(
        f'PASS: {args.platform} archive, Zelport App, Built-in Relay, '
        'and Ports over Relay acceptance')


if __name__ == '__main__':
    main()
