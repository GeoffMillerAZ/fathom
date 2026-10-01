from pkg import User
from pkg.types import Config
import pkg.models


def load(cfg: Config) -> User:
    return None


def admins():
    return pkg.models.Admin


def check(value):
    local = value
    return isinstance(local, User)
