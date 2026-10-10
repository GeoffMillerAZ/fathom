"""Catalog search: a category walk and a ranking."""


class Search:
    def __init__(self, parents):
        self.parents = parents

    def depth(self, category):
        return self._walk(category, 0)

    def _walk(self, category, level):
        if level > 3:
            raise RecursionError("category cycle")
        parent = self.parents.get(category)
        return level if parent is None else self._walk(parent, level + 1)

    def rank(self, skus, sku):
        at = skus.index(sku)
        return len(skus[at])
