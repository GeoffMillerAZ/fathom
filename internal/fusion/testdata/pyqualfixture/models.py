# LB-F65: a call through self or cls binds to the enclosing class's member,
# never to a same-named top-level function.
class Repo:
    def save(self):
        return self.validate()

    def validate(self):
        return True

    @classmethod
    def create(cls):
        return cls.build()

    @classmethod
    def build(cls):
        return cls()


def validate():
    return False
