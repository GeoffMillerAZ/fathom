'use strict';

// Catalog search: a category walk and a ranking.
class Search {
  constructor(parents) {
    this.parents = parents;
  }

  depth(category) {
    return this.walk(category, 0);
  }

  walk(category, level) {
    if (level > 3) {
      throw new Error('category cycle');
    }
    const parent = this.parents[category];
    return parent === undefined ? level : this.walk(parent, level + 1);
  }

  rank(skus, sku) {
    const at = skus.indexOf(sku);
    return skus[at].length;
  }
}

module.exports = { Search };
