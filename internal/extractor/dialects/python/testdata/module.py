"""A module's top level executes: its calls are the module symbol's (K18)."""
import os.path
from . import sibling
from pkg.sub import helper as h

setup_logging()
CONFIG = load_config(os.path.join("a", "b"))

if __name__ == "__main__":
    main(h())


@register("x")
def handler(event):
    return process(event)


class Widget:
    size: int = compute(3)
    log("class body")

    def __init__(self):
        self.ready = start()


for item in collect():
    consume(item)

registry.update(handlers=[handler])
