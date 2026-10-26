"""Observe evaluated native lexer fields without supplying parser decisions."""
import hashlib
from pathlib import Path
HERE = Path(__file__).resolve().parent
SOURCE_SHA256 = '06ebe816c356b4538f427d5581a958a31173eb3f7bc841dc7a90f53d1c9ceca7'
SIGNATURE = 'ngx_conf_read_token(ngx_conf_t *cf)\n{'
OBSERVED = 'ngx_conf_read_token_oracle_impl(ngx_conf_t *cf)\n{'
DECLARATIONS = 'static ngx_uint_t ngx_oracle_first_line;\nstatic ngx_uint_t ngx_oracle_token_count;\n'
ANCHOR = 'static ngx_int_t ngx_conf_read_token(ngx_conf_t *cf);'
HOOK_ANCHOR = '            if (found) {\n                word = ngx_array_push(cf->args);'
HOOK = ('                if (ngx_test_config && cf->args->nelts == 0) {\n'
        '                    ngx_oracle_first_line = start_line;\n'
        '                }\n')


def transform(before):
    text = before.decode()
    if text.count(SIGNATURE) != 1 or text.count(ANCHOR) != 1 or text.count(HOOK_ANCHOR) != 1:
        raise ValueError('native lexer source shape drift or double instrumentation')
    shim = (HERE / 'token_observer.c.in').read_text()
    changed = text.replace(ANCHOR, DECLARATIONS + ANCHOR)
    changed = changed.replace(SIGNATURE, OBSERVED)
    changed = changed.replace(HOOK_ANCHOR, '            if (found) {\n' + HOOK + '                word = ngx_array_push(cf->args);')
    changed += '\n' + shim
    restored = changed[:-(len(shim) + 1)].replace(DECLARATIONS, '', 1).replace(OBSERVED, SIGNATURE, 1).replace(HOOK, '', 1)
    if restored.encode() != before:
        raise ValueError('native original parser body preservation failed')
    return changed.encode(), hashlib.sha256(shim.encode()).hexdigest()


def instrument(source):
    relative = 'src/core/ngx_conf_file.c'
    path = source / relative
    before = path.read_bytes()
    if hashlib.sha256(before).hexdigest() != SOURCE_SHA256:
        raise ValueError('native lexer source identity drift')
    changed, shim = transform(before)
    path.write_bytes(changed)
    return dict(path=relative, original_sha256=SOURCE_SHA256,
                observed_sha256=hashlib.sha256(changed).hexdigest(), shim_sha256=shim,
                original_control_flow_and_return_values_preserved=True)
