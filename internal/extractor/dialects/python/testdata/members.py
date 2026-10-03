"""LB-F241 fixture: attributes and fields as Owner.name symbols, each once, at
the first definition scip-python 0.6.6 gives it."""
import os

if os.environ.get("X"):
    flag = load_flag()
else:
    flag = None
first, second = "a", "b"
head = tail = 0


def work():
    local = compute()
    return local


class Model:
    name: str
    count: int = 0
    untyped = make_default()
    c1 = c2 = 0
    t1, t2 = 1, 2
    if os.name == "nt":
        cond = 1

    def __init__(self, value):
        self.value = value
        self.client = connect()
        self.typed: int = 0
        self.a, self.b = 1, 2
        self.name = value
        self.count = 1
        local = value
        local.attr = 1

    def reset(self):
        self.client = reconnect()
        self.late = 2

        def inner():
            self.deep = 3

        return inner

    def label(self):
        return self.value

    label = property(label)

    class Inner:
        def m(self):
            self.x = 1


def outer():
    class Local:
        field: int

        def m(self):
            self.y = 1

    return Local
