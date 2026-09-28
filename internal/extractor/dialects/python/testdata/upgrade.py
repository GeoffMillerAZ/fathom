"""LB-Q-python upgrade fixture: PEP 695 type aliases, annotated class
attributes with a default value, and a decorated property pair."""

type IntOrStr = int | str


class Model:
    name: str
    count: int = 0
    untyped = 1

    @property
    def label(self):
        return self.name

    @label.setter
    def label(self, value):
        self.name = value

    @route(
        "/x",
        methods=["GET"],
    )
    def handle(self):
        call_it()
