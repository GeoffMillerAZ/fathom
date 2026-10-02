from conan import ConanFile


class Early(ConanFile):
    name = "early"
    requires = "zlib/1.3.1"


class Broken(ConanFile):
    name = "broken"
    requires = "fmt/10.2.1"

    def requirements(self:
        self.requires("openssl/3.0"
