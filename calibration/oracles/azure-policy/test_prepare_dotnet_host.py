import tarfile
import unittest
from prepare_dotnet_host import validate_members, MAX_FILES, MAX_EXPANDED


class RuntimeArchiveTest(unittest.TestCase):
    def test_only_relative_regular_files_and_directories_are_allowed(self):
        validate_members([tarfile.TarInfo('dotnet')])
        for name, kind in [('../escape', tarfile.REGTYPE), ('/absolute', tarfile.REGTYPE),
                           ('link', tarfile.SYMTYPE), ('device', tarfile.CHRTYPE)]:
            member = tarfile.TarInfo(name); member.type = kind
            with self.assertRaisesRegex(ValueError, 'unsafe'):
                validate_members([member])

    def test_count_and_expansion_cap_plus_one_refuse_whole_archive(self):
        with self.assertRaisesRegex(ValueError, 'budget'):
            validate_members([tarfile.TarInfo('dotnet')] * (MAX_FILES + 1))
        member = tarfile.TarInfo('dotnet'); member.size = MAX_EXPANDED + 1
        with self.assertRaisesRegex(ValueError, 'budget'):
            validate_members([member])
