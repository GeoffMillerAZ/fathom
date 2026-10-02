#include "api.h"

int api_open(int fd) { return fd; }

int api_close(int fd) { return api_open(fd); }

int api_wrapped(int v) { return v; }

static int hidden(int v) { return v; }
