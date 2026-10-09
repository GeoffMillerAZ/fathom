"""Authentication: tokens and password hashing."""

import hashlib


class Auth:
    def __init__(self, secret):
        self.secret = secret

    def verify(self, token):
        if not token.endswith(self.secret):
            raise PermissionError("token rejected")

    def hash_password(self, password, algorithm):
        return hashlib.new(algorithm, password.encode()).hexdigest()
