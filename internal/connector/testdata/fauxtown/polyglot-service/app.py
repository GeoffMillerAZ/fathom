"""Polyglot fixture: Python."""


class Greeter:
    def hello(self, name):
        return f"Hello, {name}"


def greet_all(names):
    return [Greeter().hello(n) for n in names]
