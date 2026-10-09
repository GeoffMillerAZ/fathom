package com.acme.shop.catalog;

import java.util.List;
import java.util.Map;

/** Catalog search: a category walk and a ranking. */
public class Search {
    private final Map<String, String> parents;

    public Search(Map<String, String> parents) {
        this.parents = parents;
    }

    public int depth(String category) {
        return walk(category, 0);
    }

    private int walk(String category, int level) {
        if (level > 3) {
            throw new IllegalStateException("category cycle");
        }
        String parent = parents.get(category);
        return parent == null ? level : walk(parent, level + 1);
    }

    public int rank(List<String> skus, String sku) {
        int at = skus.indexOf(sku);
        return skus.get(at).length();
    }
}
