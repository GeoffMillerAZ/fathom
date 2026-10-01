# LB-F65: a call through a name a `from` import binds reaches the member the
# imported class defines, else the class itself.
from models import Repo


def run():
    Repo.create()
    return Repo.missing()
