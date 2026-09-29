# Target-specific variable assignments.
CFLAGS = -O2

test: CFLAGS = -g
%.o: WARN := 1

$(OBJS): private CFLAGS += -Wall

a.o b.o \
c.o: private CFLAGS += -O3

real: dep
	echo real
