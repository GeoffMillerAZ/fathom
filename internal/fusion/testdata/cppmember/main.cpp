// main.cpp: d.day() and p->count() are members of the receiver's class, not
// the header's day or count; count(1) is the header's. buffer.resize() is a
// member of the receiver's class too, though the receiver is named like the
// header's cal::buffer, or like its top-level widget; cal::level() is the
// header's.
#include "calendar.hpp"

struct date {
    unsigned day() const { return 1; }
};

struct tally {
    int count() const { return 2; }
};

int use(const date &d, const tally *p) {
    return d.day() + p->count();
}

int total() { return count(1); }

struct text {
    void resize(unsigned) {}
};

int grow(text &buffer) {
    buffer.resize(4);
    return 0;
}

int clear() { return cal::level(); }

int shrink(text &widget) {
    widget.resize(1);
    return 0;
}
