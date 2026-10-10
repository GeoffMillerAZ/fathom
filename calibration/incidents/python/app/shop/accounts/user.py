"""A user account."""


class User:
    def __init__(self, name, email):
        self.name = name
        self.email = email

    def rename(self, full):
        first, last = full.split(" ", 1)
        self.name = first + " " + last.upper()

    def domain(self):
        parts = self.email.split("@")
        return parts[1].lower()
