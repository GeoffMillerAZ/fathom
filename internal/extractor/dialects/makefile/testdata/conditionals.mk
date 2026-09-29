# Conditional variables and rules, and target-specific variables.
ifeq ($(OS),Windows_NT)
EXE = .exe
else
EXE =
endif

ifdef VERBOSE
Q =
else
Q = @
endif

build: deps
	$(Q)echo building

ifneq ($(CI),)
ci-report: build
	echo report
endif

test: CFLAGS = -g
test: build
	$(Q)./run-tests

deps:
	echo deps
