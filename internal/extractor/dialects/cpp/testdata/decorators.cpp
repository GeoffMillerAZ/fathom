// decorators.cpp — a library's export, inline and noexcept spellings as
// object-like macros, and its try and catch as macros (O455, the cpp-macros
// pre-pass). Every definition is named as the compiler, after expansion,
// names it; the comment's "class LIB_API" and the string below stay as written.
#include <string>

#define LIB_API
#define LIB_INLINE inline
#define LIB_NOEXCEPT noexcept
#define LIB_CONSTEXPR constexpr
#define LIB_TRY try
#define LIB_CATCH(x) catch (x)
#define LIB_BEGIN_NAMESPACE namespace lib {
#define LIB_END_NAMESPACE }
#define LIB_ENABLE_IF(...) int = 0
#define LIB_PRIVATE_UNLESS_TESTED private

LIB_BEGIN_NAMESPACE

class LIB_API logger final : public sink {
public:
    LIB_API explicit logger(std::string name);
    LIB_API ~logger();
    void log(const std::string &msg) LIB_NOEXCEPT;

LIB_PRIVATE_UNLESS_TESTED:
    std::string name_;
};

struct LIB_API record {
    int level;
};

template <typename Char>
#if defined(LIB_WIDE)
using string_type = std::basic_string<Char>;
#else
using string_type = std::string;
#endif

LIB_INLINE logger::logger(std::string name) : name_(name) {}

LIB_INLINE void logger::log(const std::string &msg) LIB_NOEXCEPT {
    LIB_TRY {
        write(msg);
    }
    LIB_CATCH(const std::exception &e) {
        report(e.what());
    }
}

std::string LIB_INLINE level_name(int level) {
    const char *label = "class LIB_API kept";
    return names(level, label);
}

template <typename T, LIB_ENABLE_IF(sizeof(T) > 1)>
LIB_CONSTEXPR LIB_INLINE T widen(T v) {
    if LIB_CONSTEXPR (sizeof(T) > 4) {
        return v;
    }
    return convert(v);
}

LIB_END_NAMESPACE
