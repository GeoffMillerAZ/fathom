import unittest
from native_definitions import definitions
from native_tokens import decode, trees
from test_native_tokens import emit


class NativeDefinitionsTest(unittest.TestCase):
    def test_complete_native_population_keeps_unused_entities_and_exact_ranges(self):
        raw = (emit(1, 'block', 2, 2, ['http']) + emit(2, 'block', 3, 4, ['upstream','unused.invalid']) +
               emit(3,'simple',5,5,['server','127.0.0.1:19001']) + emit(4,'end',0,6) +
               emit(5,'block',7,7,['server']) + emit(6,'simple',8,8,['server_name','a.invalid']) +
               emit(7,'block',9,10,['location','/']) + emit(8,'end',0,11) + emit(9,'end',0,12) +
               emit(10,'end',0,13) + emit(11,'file',0,14,last_lf=1))
        result = definitions('authored.conf', trees(decode(raw))['authored.conf'])
        self.assertEqual(5,len(result))
        self.assertIn(dict(path='authored.conf',name='unused.invalid',kind='service',line=3,end=6),result)
        self.assertIn(dict(path='authored.conf',name='a.invalid /',kind='route',line=9,end=11),result)
        self.assertEqual(13,result[0]['end'])

    def test_data_entries_are_not_silently_promoted_to_definitions(self):
        native = dict(file_end=4,tree=[dict(directive='map',args=['$x','$y'],line=1,end=4,block=[dict(directive='server',args=['text'],line=2,end=2)])])
        self.assertEqual(1,len(definitions('authored.conf',native)))

    def test_private_path_in_native_entity_refuses_instead_of_guessing_original(self):
        native = dict(file_end=3,tree=[dict(directive='upstream',args=['x'],line=1,end=3,block=[dict(directive='server',args=['unix:/private/profile/backend'],line=2,end=2)])])
        with self.assertRaisesRegex(ValueError,'isolation'):
            definitions('authored.conf',native,'/private/profile')
