import unittest
from population import decode_events, local_population, MAX_EVENTS, MAX_STDERR_BYTES


def native_line(domain, pointer, flags, line, path, command, argument, host):
    fields = [domain, format(pointer, 'x'), str(flags), str(line)]
    fields += [value.encode().hex() for value in (path, command, argument, host)]
    return 'nginx: ORACLE_NGINX_UPSTREAM_V1 ' + ' '.join(fields)


class NativePopulationTest(unittest.TestCase):
    def events(self):
        # Forward reference, unused declaration and both native API domains.
        return decode_events('\n'.join([
            native_line('http', 1, 0, 7, 'site.conf', 'proxy_pass', 'http://app', 'app'),
            native_line('http', 1, 1, 9, 'site.conf', 'upstream', 'app', 'app'),
            native_line('http', 2, 1, 12, 'site.conf', 'upstream', 'unused', 'unused'),
            native_line('stream', 3, 1, 15, 'site.conf', 'upstream', 'stream_app', 'stream_app'),
            native_line('stream', 3, 0, 18, 'site.conf', 'proxy_pass', 'stream_app', 'stream_app')]))

    def test_native_identity_enumerates_pairs_without_scanner_candidates(self):
        refs, audit = local_population(self.events(), 'site.conf', lambda event: 'owner-' + str(event['line']))
        self.assertEqual([('owner-18', 'stream_app'), ('owner-7', 'app')], [(r['from'], r['to']) for r in refs])
        self.assertEqual(3, audit['declarations'])
        self.assertEqual(2, audit['native_local_sites'])

    def test_unknown_native_events_and_missing_owners_fail_closed(self):
        events = self.events()
        events[-1]['command'] = 'unknown_pass'
        with self.assertRaisesRegex(ValueError, 'unclassified'):
            local_population(events, 'site.conf', lambda event: 'owner')
        with self.assertRaises(ValueError):
            local_population(self.events(), 'site.conf', lambda event: None)

    def test_native_target_identity_wins_over_name_guessing(self):
        events = self.events()
        events[0]['host'] = 'different-spelling'
        refs, _ = local_population(events, 'site.conf', lambda event: 'owner')
        self.assertEqual('app', refs[0]['to'])

    def test_malformed_or_incomplete_native_records_are_refused(self):
        with self.assertRaises(ValueError):
            decode_events('nginx: ORACLE_NGINX_UPSTREAM_V1 missing fields')
        with self.assertRaises(ValueError):
            decode_events('nginx: ORACLE_NGINX_UPSTREAM_ERROR allocation')
        with self.assertRaises(ValueError):
            decode_events(native_line('http', 0, 0, 1, 'site.conf', 'proxy_pass', 'http://app', 'app'))

    def test_empty_native_host_is_retained_as_an_implicit_site(self):
        events = decode_events(native_line('http', 4, 0, 20, 'site.conf', 'fastcgi_pass', 'unix:/socket', ''))
        refs, audit = local_population(events, 'site.conf', lambda event: 'owner')
        self.assertEqual([], refs)
        self.assertEqual(1, audit['implicit_or_external_sites'])

    def test_event_cap_plus_one_refuses_whole_profile(self):
        row = native_line('http', 1, 0, 1, 'site.conf', 'proxy_pass', 'http://app', 'app')
        self.assertEqual(MAX_EVENTS, len(decode_events('\n'.join([row] * MAX_EVENTS))))
        with self.assertRaisesRegex(ValueError, 'event count'):
            decode_events('\n'.join([row] * (MAX_EVENTS + 1)))

    def test_stderr_cap_plus_one_refuses_whole_profile(self):
        self.assertEqual([], decode_events('x' * MAX_STDERR_BYTES))
        with self.assertRaisesRegex(ValueError, 'stderr'):
            decode_events('x' * (MAX_STDERR_BYTES + 1))
