import unittest
from native_tokens import MAX_ARGS, MAX_STDERR, MAX_TOKENS, MAX_WORD, decode, trees


def emit(ident, kind, start, end, args=(), path='authored.conf', last_lf=0):
    header = 'ORACLE_NGINX_TOKEN_V1 %d %s %d %d %d %d %s\n' % (ident, kind, start, end, last_lf, len(args), path.encode().hex())
    return (header + ''.join('ORACLE_NGINX_ARG_V1 %d %d %s\n' % (ident, index, arg.encode().hex() or '-') for index, arg in enumerate(args))).encode()


class NativeTokenReaderTest(unittest.TestCase):
    def test_complete_native_tree_has_multiline_header_and_closing_range(self):
        raw = emit(1, 'block', 2, 4, ['http']) + emit(2, 'simple', 5, 6, ['empty', '']) + emit(3, 'end', 0, 8) + emit(4, 'file', 0, 10, last_lf=1)
        tree = trees(decode(raw))['authored.conf']
        self.assertEqual(9, tree['file_end'])
        self.assertEqual(2, tree['tree'][0]['line'])
        self.assertEqual(8, tree['tree'][0]['end'])
        self.assertEqual([''], tree['tree'][0]['block'][0]['args'])

    def test_native_only_tokens_remain_without_scanner_input(self):
        raw = emit(1, 'simple', 1, 1, ['unused', 'café']) + emit(2, 'file', 0, 1)
        self.assertEqual('unused', trees(decode(raw))['authored.conf']['tree'][0]['directive'])

    def test_all_reader_caps_refuse_whole_profile(self):
        for raw in [b'x' * (MAX_STDERR + 1), emit(1, 'simple', 1, 1, ['x'] * (MAX_ARGS + 1)),
                    emit(1, 'simple', 1, 1, ['x' * (MAX_WORD + 1)]),
                    b''.join(emit(n, 'file', 0, 1, path=str(n)) for n in range(1, MAX_TOKENS + 2))]:
            with self.assertRaisesRegex(ValueError, 'cap'):
                decode(raw)

    def test_partial_corrupt_unknown_or_refused_events_never_become_truth(self):
        for raw in [emit(1, 'simple', 1, 1, ['x']).split(b'ORACLE_NGINX_ARG')[0],
                    emit(1, 'unexpected', 1, 1), b'ORACLE_NGINX_TOKEN_REFUSED argument_cap\n',
                    emit(2, 'file', 0, 1)]:
            with self.assertRaises(ValueError):
                decode(raw)

    def test_missing_block_or_file_boundaries_refuse(self):
        for raw in [emit(1, 'block', 1, 1, ['http']), emit(1, 'end', 0, 1) + emit(2, 'file', 0, 1),
                    emit(1, 'block', 1, 1, ['http']) + emit(2, 'file', 0, 1)]:
            with self.assertRaises(ValueError):
                trees(decode(raw))

    def test_identical_repeats_retain_all_evaluation_counts(self):
        raw = emit(1,'simple',1,1,['x']) + emit(2,'file',0,1) + emit(3,'simple',1,1,['x']) + emit(4,'file',0,1)
        result = trees(decode(raw))['authored.conf']
        self.assertEqual(2,result['evaluations'])
        self.assertEqual(2,result['token_count'])
        self.assertEqual(1,len(result['tree']))
        changed = raw.replace(b'ORACLE_NGINX_ARG_V1 3 0 78',b'ORACLE_NGINX_ARG_V1 3 0 79')
        with self.assertRaisesRegex(ValueError,'identity'):
            trees(decode(changed))

    def test_unknown_protocol_record_cannot_be_ignored_beside_valid_population(self):
        raw = emit(1,'file',0,1) + b'ORACLE_NGINX_TOKEN_V2 unknown\n'
        with self.assertRaisesRegex(ValueError,'protocol'):
            decode(raw)
