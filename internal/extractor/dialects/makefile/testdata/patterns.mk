# Pattern, suffix, static-pattern and variable-named rules.
BUILD := out
OBJS := a.o b.o
HDRS = defs.h
NAME = core

$(BUILD)/main.o: src/main.c $(HDRS) lib/$(NAME).h
	$(CC) -c $< -o $@

%.o: %.c
	$(CC) -c $<

.c.o:
	$(CC) -c $<

$(OBJS): %.o: %.c
	$(CC) -c $< -o $@

lib.a:: a.o
	ar rcs $@ $^

lib.a:: b.o
	ar rcs $@ $^

gen-%.h: gen.sh
	./gen.sh $* > $@

all: $(addprefix $(BUILD)/,main.o) lib.a
