from conan import ConanFile
from conan.tools.cmake import CMake, cmake_layout


class DemoConan(ConanFile):
    name = "demo"
    version = "1.2.3"
    license = "MIT"
    settings = "os", "compiler", "build_type", "arch"
    options = {"shared": [True, False], "with_ssl": [True, False]}
    default_options = {"shared": False, "with_ssl": True}
    requires = "zlib/[>=1.2.11 <2]"
    tool_requires = "cmake/3.27.0"
    python_requires = "base_recipe/2.0@acme/stable"

    def requirements(self):
        self.requires("fmt/10.2.1", transitive_headers=True)
        if self.options.with_ssl:
            self.requires(f"openssl/[>=3 <4]")
        self.test_requires("gtest/1.14.0")

    def build_requirements(self):
        self.tool_requires("ninja/[>=1.10 <2]")

    def layout(self):
        cmake_layout(self)

    def package_info(self):
        self.cpp_info.libs = ["demo"]
        self.cpp_info.components["core"].requires.append("zlib::zlib")
