// calendar.hpp: a class and a free function named like members other
// classes have.
#pragma once

class day {
public:
    explicit day(unsigned d) : d_(d) {}
    unsigned value() const { return d_; }

private:
    unsigned d_;
};

inline int count(int n) { return n; }
