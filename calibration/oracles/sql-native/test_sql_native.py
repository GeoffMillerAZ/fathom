import unittest
import sql_native as oracle


class SQLiteCatalogTests(unittest.TestCase):
    def test_failed_data_copy_keeps_native_ddl_truth_and_later_statements(self):
        result = oracle.sqlite_file('upgrade.sql', '''BEGIN;
CREATE TABLE new_changes (id TEXT REFERENCES projects(id));
INSERT INTO new_changes SELECT * FROM changes;
CREATE TABLE new_events (id TEXT REFERENCES projects(id));
COMMIT;
''')
        self.assertEqual(result['calls'], [
            {'path': 'upgrade.sql', 'from': 'new_changes', 'to': 'projects'},
            {'path': 'upgrade.sql', 'from': 'new_events', 'to': 'projects'},
        ])
        self.assertEqual(len(result['errors']), 1)
        self.assertEqual(result['create_table_failed'], 0)
        self.assertEqual(result['statements'], 5)

    def test_rejected_create_table_is_not_an_empty_success(self):
        result = oracle.sqlite_file('bad.sql', 'CREATE TABLE child (id INTEGER REFERENCES);')
        self.assertEqual(result['create_table_failed'], 1)
        self.assertEqual(len(result['errors']), 1)
        self.assertEqual(result['calls'], [])

    def test_trigger_and_quoted_semicolons_do_not_split_statements(self):
        sql = '''CREATE TABLE parent(id);
CREATE TABLE child(id REFERENCES parent, note DEFAULT ';');
CREATE TRIGGER log AFTER INSERT ON parent BEGIN
 INSERT INTO child VALUES (new.id, ';');
 INSERT INTO child VALUES (new.id, 'x');
END;'''
        result = oracle.sqlite_file('trigger.sql', sql)
        self.assertEqual(result['statements'], 3)
        self.assertEqual(result['errors'], [])
        self.assertEqual(result['calls'], [{'path': 'trigger.sql', 'from': 'child', 'to': 'parent'}])

    def test_preserves_source_slices_and_forbids_filesystem_attach(self):
        sql = "-- lead\nCREATE TABLE p(id);\nATTACH DATABASE '/nonexistent/probe' AS foreign_db;"
        pieces = list(oracle.sqlite_statements(sql))
        self.assertEqual(''.join(x[1] for x in pieces), sql)
        result = oracle.sqlite_file('attach.sql', sql)
        self.assertTrue(any(x['code'] == 'SQLITE_AUTH' for x in result['errors']))

    def test_rename_does_not_create_new_pairs(self):
        result = oracle.sqlite_file('rename.sql', 'CREATE TABLE a(id REFERENCES b); ALTER TABLE a RENAME TO c;')
        self.assertEqual(result['calls'], [{'path': 'rename.sql', 'from': 'a', 'to': 'b'}])

class SourcePreservationTests(unittest.TestCase):
    def test_crlf_is_preserved_in_original_statement_slices(self):
        sql = '-- comment\r\nCREATE TABLE p(id);\r\nCREATE TABLE c(id REFERENCES p);\r\n'
        self.assertEqual(''.join(text for _, text in oracle.sqlite_statements(sql)), sql)
        self.assertEqual(list(oracle.sqlite_statements(sql))[0][0], 2)


class TemporaryTableTests(unittest.TestCase):
    def test_temporary_table_keys_are_observed_too(self):
        r = oracle.sqlite_file('temp.sql', 'CREATE TEMP TABLE child(id REFERENCES parent);')
        self.assertEqual(r['calls'], [{'path': 'temp.sql', 'from': 'child', 'to': 'parent'}])


if __name__ == '__main__':
    unittest.main()
