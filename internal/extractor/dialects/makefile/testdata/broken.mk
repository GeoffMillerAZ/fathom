# A syntax error must not lose the rules around it.
GOOD = 1

before: dep
	echo before

)))

after: dep2
	echo after

LATE = 2
