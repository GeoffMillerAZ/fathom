// main.cpp: d.day() and p->count() are members of the receiver's class, not
// the header's day or count; count(1) is the header's.
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
