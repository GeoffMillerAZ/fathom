"""Weight helpers for shipping: the same basename as billing's util."""


def round_cents(grams, step):
    boxes = grams // step
    return boxes * step


def bands():
    return [500, 1000, 2000]
