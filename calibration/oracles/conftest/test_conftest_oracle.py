import unittest
import conftest_oracle as o
class PositionsTest(unittest.TestCase):
 def test_multiline_values_do_not_invent_keys(self):
  self.assertEqual(o.positions('policy = [\n "checks",\n "shared"\n]\nnamespace = "main"\n'),{'policy':(1,4),'namespace':(5,5)})
 def test_strings_and_quoted_keys(self):
  self.assertEqual(o.positions('"fail-on-warn" = true\nnamespace = """\nnot_a_key = 1\n"""\n'),{'fail-on-warn':(1,1),'namespace':(2,4)})
 def test_malformed_rejected(self):
  with self.assertRaises(ValueError):o.positions('policy = [ }')
if __name__=='__main__':unittest.main()
