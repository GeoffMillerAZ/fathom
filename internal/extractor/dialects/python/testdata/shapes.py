"""Module docstring."""
import os
import os.path as osp
from collections import OrderedDict, defaultdict as dd
from . import sibling
from .pkg.mod import thing as other

LIMIT = 10
name, other_name = "a", "b"
counter: int = 0


def top(a, b=2, *args, **kwargs):
    """Top-level function."""
    helper(a)
    os.path.join(a, b)
    return [x for x in args if check(x)]


async def fetch(url):
    await client.get(url)


@decorator
def decorated(
    first,
    second,
):
    return first + second


class Base:
    attr = 1

    def __init__(self, value):
        self.value = value
        super().__init__()

    @property
    def size(self):
        return len(self.value)

    @staticmethod
    def build(*parts):
        return Base(parts)

    class Inner:
        def deep(self):
            def local():
                return compute()
            return local()


class Child(Base, metaclass=Meta):
    def method(self):
        self.build().size
        Base.build(1, 2)


def outer():
    class Local:
        def m(self):
            pass
    return Local
