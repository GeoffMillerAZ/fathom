# Continued prerequisites, inline recipes and canned recipes.
SRCS = main.c \
       util.c

bundle: main.o \
        util.o \
        | outdir
	$(CC) -o $@ main.o util.o

inline: dep ; echo inline

recurse:
	$(MAKE) -C sub all

define compile_rule
$(1).o: $(1).c
	$(CC) -c $$<
endef

$(foreach s,$(SRCS),$(eval $(call compile_rule,$(basename $(s)))))
