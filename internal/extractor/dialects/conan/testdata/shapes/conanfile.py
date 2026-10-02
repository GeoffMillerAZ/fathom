from conan import ConanFile
import conan


class ListsConan(conan.ConanFile):
    name = "lists"
    requires = ["a/1.0", "b/[~2.1]#abcdef", "c/3.0@user/channel"]
    build_requires = ("d/1.0", "e/2.0")
    test_requires = ("f/0.1",)

    def requirements(self):
        for dep in ("g/1.0",):
            self.requires(dep)
        self.requires(self.tested_reference_str)
        self.requires("h/1.0", options={"x": True})
        self.build_requires("i/1.0")


class Helper:
    name = "not-a-recipe"
    requires = "j/1.0"

    def run(self):
        self.requires("k/1.0")


class NoName(ConanFile):
    requires = "l/1.0"
