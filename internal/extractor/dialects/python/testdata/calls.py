# Call sites under calls.qualify (LB-F65): the receiver chain stays, argument
# and index contents go, a callee with no name of its own is not stored, and a
# call outside every function belongs to the definition it decorates or defines.
import functools

registry = functools.partial(dict, a=1)


@functools.lru_cache(maxsize=None)
def cached(key):
    rows = db.query(Item).filter(
        Item.id == key
    ).all()
    handlers[key](rows)
    make()()
    return helper(rows)


class Service:
    limit: int = compute_limit(3)

    @staticmethod
    @trace(level=1)
    def build(self):
        return self.make()
