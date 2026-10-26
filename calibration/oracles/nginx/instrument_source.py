"""Observe the native HTTP/stream upstream registration APIs, without re-parsing.

The original function bodies and return values are untouched. A wrapper records
native parser arguments, source location, declaration flags and returned object
identity for every call during config testing. Whole-run success is required.
"""
import hashlib
from pathlib import Path

HERE = Path(__file__).resolve().parent


def instrument(source):
    results = []
    for domain, relative in (('http', 'src/http/ngx_http_upstream.c'),
                             ('stream', 'src/stream/ngx_stream_upstream.c')):
        path = source / relative
        before = path.read_bytes()
        text = before.decode()
        typename = 'ngx_' + domain + '_upstream_srv_conf_t'
        function = 'ngx_' + domain + '_upstream_add'
        if function + '_oracle_impl(' in text:
            raise ValueError('native source is already instrumented')
        signature = typename + ' *\n' + function + '(ngx_conf_t *cf, ngx_url_t *u, ngx_uint_t flags)'
        if text.count(signature) != 1:
            raise ValueError('native source signature drift')
        changed = text.replace(signature, 'static ' + typename + ' *\n' + function + '_oracle_impl(ngx_conf_t *cf, ngx_url_t *u, ngx_uint_t flags)')
        observer = (HERE / 'observer_shim.c.in').read_text().replace('@TYPE@', typename).replace('@FUNCTION@', function).replace('@DOMAIN@', domain)
        changed += '\n' + observer
        path.write_text(changed)
        results.append(dict(path=relative, original_sha256=hashlib.sha256(before).hexdigest(),
                            observed_sha256=hashlib.sha256(changed.encode()).hexdigest()))
    return dict(kind='native parser-call population observer; no source-walk selection',
                shim_sha256=hashlib.sha256((HERE / 'observer_shim.c.in').read_bytes()).hexdigest(), sources=results)
