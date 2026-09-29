# Basic variables, targets, and recipes.
CC = gcc
CFLAGS := -O2 -Wall
LIBS += -lm
DEBUG ?= 0
STAMP != date +%s
VERSION ::= 1.2
override V = 1
export PATH_X ?= /usr
export EXPORTED
.DEFAULT_GOAL := all

.PHONY: all clean

all: prog docs

prog: main.o util.o | outdir
	$(CC) $(CFLAGS) -o $@ $^ $(LIBS)

main.o: main.c util.h
	$(CC) -c $<

util.o: util.c util.h
	$(CC) -c $<

docs: readme.txt

outdir:
	mkdir -p $@

clean:
	rm -f *.o prog

a b: shared
	touch $@

define greet
echo hello $(1)
endef
